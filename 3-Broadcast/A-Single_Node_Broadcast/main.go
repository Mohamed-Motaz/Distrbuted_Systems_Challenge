package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

// go install . && ~/Downloads/maelstrom/maelstrom test -w broadcast --bin ~/go/bin/maelstrom-broadcast --node-count 1 --time-limit 20 --rate 10

//Since we have a known number of nodes, we can simply allocate of chunk of ids that each
//node is allowed to return

var (
	messagesMu       sync.Mutex
	receivedMessages []int

	topoMu   sync.Mutex
	topology map[string][]string
)

func Init() {
	messagesMu = sync.Mutex{}
	receivedMessages = make([]int, 0)

	topoMu = sync.Mutex{}
	topology = make(map[string][]string)
}

func main() {
	Init()

	n := maelstrom.NewNode()

	n.Handle("broadcast", func(msg maelstrom.Message) error {

		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "broadcast_ok"

		//messagesMu.Lock()
		receivedMessages = append(receivedMessages, int(body["message"].(float64)))
		//messagesMu.Unlock()

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
		body["messages"] = receivedMessages
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
		t := body["topology"].(map[string]interface{})
		topology = make(map[string][]string) //clear the map
		for k, v := range t {
			interfacesArr := v.([]interface{})
			topology[k] = make([]string, 0)

			for _, v := range interfacesArr {
				topology[k] = append(topology[k], v.(string))
			}
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
