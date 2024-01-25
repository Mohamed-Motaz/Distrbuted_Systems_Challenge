package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

// go install . && ~/Downloads/maelstrom/maelstrom test -w broadcast --bin ~/go/bin/maelstrom-broadcast --node-count 5 --time-limit 20 --rate 10 --nemesis partition

//this solution contains stale data

//Solution is very simple
//Simply broadcast to all nodes in the topology, (not all neighbors)
//and each node is responsible for gossiping to its neighbors
//This cycle stops when a node receives a message which it already has seen before,
//and at this points it stops sending this message
//Also need to keep track of which broadcasts have been acked, to retry those that haven't

var (
	node *maelstrom.Node

	messagesMu       sync.Mutex
	receivedMessages Set[int]

	topoMu   sync.Mutex
	topology []string

	broadcastChan chan (string) //msg-node || msg-*
)

func Init() {

	node = maelstrom.NewNode()

	messagesMu = sync.Mutex{}
	receivedMessages = MakeSet[int]()

	topoMu = sync.Mutex{}
	topology = make([]string, 0)

	broadcastChan = make(chan string, 100000)
}

func GetBroadcastKey(msg int, node string) string {
	return fmt.Sprintf("%d-%s", msg, node)
}

// this is a worker thread that receives messages to broadcast,
// and is responsible for keeping track of notifying the neighbors
func broadcaster(n *maelstrom.Node) {

	//start 1 thread to listen on broadcastChan and broadcast
	for {
		msg := <-broadcastChan
		arr := strings.Split(msg, "-")
		val, _ := strconv.Atoi(arr[0])
		dst := []string{arr[1]}

		if dst[0] == "*" { //send to all nodes
			dst = topology
		}
		fmt.Fprintf(os.Stderr, "About to send to %+v\n", dst)
		for _, node := range dst {

			//for every "network request", spawn a new goroutine to handle it
			go func(node string, val int) {
				var body map[string]any = map[string]any{
					"type":    "broadcast",
					"message": val,
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(100)*time.Millisecond)
				_, err := n.SyncRPC(ctx, node, body)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Unable to send message %+v to %+v with err: %+v\n", val, node, err)
					broadcastChan <- GetBroadcastKey(val, node) //re-add it to the channel
				}

				cancel()
			}(node, val)

		}
	}

}

func main() {
	Init()

	go broadcaster(node)

	node.Handle("broadcast", broadcastHandler)
	node.Handle("read", readHandler)
	node.Handle("topology", topologyHandler)

	if err := node.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

}

func broadcastHandler(msg maelstrom.Message) error {

	// Unmarshal the message body as an loosely-typed map.
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	// Update the message type to return back.
	body["type"] = "broadcast_ok"

	shouldBroadcast := false
	messagesMu.Lock()
	if !receivedMessages.Exists(int(body["message"].(float64))) {
		receivedMessages.Add(int(body["message"].(float64)))
		shouldBroadcast = true
	}
	messagesMu.Unlock()

	if shouldBroadcast {
		broadcastChan <- GetBroadcastKey(int(body["message"].(float64)), "*")
	}

	delete(body, "message")

	// Echo the original message back with the updated message type.
	return node.Reply(msg, body)
}

func readHandler(msg maelstrom.Message) error {

	// Unmarshal the message body as an loosely-typed map.
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	// Update the message type to return back.
	body["type"] = "read_ok"

	messagesMu.Lock()
	body["messages"] = receivedMessages.GetKeys()
	messagesMu.Unlock()

	// Echo the original message back with the updated message type.
	return node.Reply(msg, body)
}

func topologyHandler(msg maelstrom.Message) error {

	// Unmarshal the message body as an loosely-typed map.
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	// Update the message type to return back.
	body["type"] = "topology_ok"

	topoMu.Lock()
	tArr := body["topology"].(map[string]interface{})[node.ID()].([]interface{})
	topology = make([]string, 0) //clear the topology
	for _, v := range tArr {
		topology = append(topology, v.(string))
	}
	topoMu.Unlock()

	delete(body, "topology")
	// Echo the original message back with the updated message type.
	return node.Reply(msg, body)
}
