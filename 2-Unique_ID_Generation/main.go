package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

// go install . && ~/Downloads/maelstrom/maelstrom test -w unique-ids --bin ~/go/bin/maelstrom-unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition

//Since we have a known number of nodes, we can simply allocate of chunk of ids that each
//node is allowed to return

var UNIQUE_ID atomic.Int64 = atomic.Int64{}
var once sync.Once = sync.Once{}

// Each node has a different start range
// assumption is that number of ids per node will never be exhausted
// and will therefore always be valid
func Init(n *maelstrom.Node) {
	id, _ := strconv.Atoi(n.ID()[1:])
	start := math.MaxInt64 * (float64(id) / float64(len(n.NodeIDs())))
	UNIQUE_ID.Store(int64(start))
}

func main() {

	n := maelstrom.NewNode()

	n.Handle("generate", func(msg maelstrom.Message) error {
		once.Do(func() {
			Init(n)
		})

		// Unmarshal the message body as an loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "generate_ok"
		body["id"] = fmt.Sprintf("%d", IncrAndGetId()) //have to stringify the id because json can't deal with large numbers

		// Echo the original message back with the updated message type.
		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

}

func IncrAndGetId() int64 {
	UNIQUE_ID.Add(1)
	return UNIQUE_ID.Load()
}
