package NodeManage

import (
	message "Gossip/MessageManage"
	"context"
	"encoding/json"
)

func (n *GossipNode[T]) sendAckToNeighbors(hash string) {
	ack := message.GossipAck{Hash: hash, FromHash: n.NodeHash}
	data, _ := json.Marshal(ack)
	for _, neighbor := range n.Neighbors {
		go n.Transport.SendAck(context.Background(), neighbor, data)
	}
}

func (n *GossipNode[T]) broadcastToTargets(msg message.GossipMessage[T]) {
	state, _ := n.Storage.GetStates(msg.Hash)
	targets := n.Router.SelectTargets(n.Cost, n.Fanout, state)
	data, _ := json.Marshal(msg)

	for _, t := range targets {
		if !n.Storage.GetState(msg.Hash, t) {
			n.Storage.MarkSent(msg.Hash, t)
			go n.Transport.SendMessage(context.Background(), n.Neighbors[t], data)
		}
	}
}
