package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

// go install . && ~/Downloads/maelstrom/maelstrom test -w broadcast --bin ~/go/bin/maelstrom-broadcast --node-count 5 --time-limit 20 --rate 10

//Solution is very simple
//Simply broadcast to all nodes in the topology, (not all neighbors)
//and each node is responsible for gossiping to its neighbors
//This cycle stops when a node receives a message which it already has seen before,
//and at this points it stops sending this message

var (
	messagesMu       sync.Mutex
	receivedMessages Set

	topoMu   sync.Mutex
	topology []string

	broadcastChan chan (int)
)

func Init() {
	messagesMu = sync.Mutex{}
	receivedMessages = MakeSet()

	topoMu = sync.Mutex{}
	topology = make([]string, 0)

	broadcastChan = make(chan int, 100000)
}

// this is a worker thread that receives messages to broadcast,
// and is responsible for keeping track of notifying the neighbors
func broadcaster(n *maelstrom.Node) {

	for i := 0; i < 10; i++ {
		//start 10 threads to listen on broadcastChan and broadcast
		go func() {
			for {
				msg := <-broadcastChan

				for _, node := range topology {
					var body map[string]any = map[string]any{
						"type":    "broadcast",
						"message": msg,
					}
					if err := n.RPC(node, body, sendBroadcastPrivateHandler); err != nil {
						//re-add this message to the channel to attempt to rebroadcast
						go func(msg int) {
							time.Sleep(100 * time.Millisecond)
							broadcastChan <- msg
						}(msg)
					}
				}
			}

		}()
	}

}

func sendBroadcastPrivateHandler(msg maelstrom.Message) error {
	return nil
}

func main() {
	Init()

	n := maelstrom.NewNode()
	broadcaster(n)

	n.Handle("broadcast", func(msg maelstrom.Message) error {

		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "broadcast_ok"

		shouldBroadcast := false
		messagesMu.Lock()
		if !Exists(receivedMessages, int(body["message"].(float64))) {
			Add(receivedMessages, int(body["message"].(float64)))
			shouldBroadcast = true
		}
		messagesMu.Unlock()

		if shouldBroadcast {
			broadcastChan <- int(body["message"].(float64))
		}

		delete(body, "message")

		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})

	n.Handle("read", func(msg maelstrom.Message) error {

		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "read_ok"

		messagesMu.Lock()
		body["messages"] = GetKeys(receivedMessages)
		messagesMu.Unlock()

		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})

	n.Handle("topology", func(msg maelstrom.Message) error {

		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "topology_ok"

		topoMu.Lock()
		tArr := body["topology"].(map[string]interface{})[n.ID()].([]interface{})
		topology = make([]string, 0) //clear the topology
		for _, v := range tArr {
			topology = append(topology, v.(string))
		}
		topoMu.Unlock()

		delete(body, "topology")
		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

}
