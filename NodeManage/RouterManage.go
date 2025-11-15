package NodeManage

import (
	message "Gossip/MessageManage"
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (n *GossipNode[T]) RegisterRoutes(r *gin.Engine) {
	r.POST("/RecieveMessage", n.HandleReceiveMessage)
	r.POST("/RecieveAck", n.HandleReceiveAck)
}

func keys[T comparable](m map[T]string) []T {
	ks := make([]T, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// ==========================
// Handle Receive Message
// ==========================
func (n *GossipNode[T]) HandleReceiveMessage(c *gin.Context) {
	var msg message.GossipMessage[T]
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if n.Storage.Has(msg.Hash) {
		c.JSON(http.StatusOK, gin.H{"status": "repeated"})
		return
	}

	n.Storage.Mark(msg.Hash)
	n.Storage.InitState(msg.Hash, keys(n.Neighbors))
	log.Println("[DEBUG] MESSAGE RECEIVED | ", msg.Hash)

	// 发送 ACK
	go n.sendAckToNeighbors(msg.Hash)

	// 异步广播
	go n.broadcastToTargets(msg)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

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

// ==========================
// Handle Receive ACK
// ==========================
func (n *GossipNode[T]) HandleReceiveAck(c *gin.Context) {
	var ack message.GossipAck
	if err := c.ShouldBindJSON(&ack); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	n.Storage.UpdateState(ack.Hash, ack.FromHash)

	state, _ := n.Storage.GetStates(ack.Hash)
	allOK := true
	for _, v := range state {
		if !v {
			allOK = false
			break
		}
	}

	if allOK {
		n.Storage.DeleteState(ack.Hash)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ack received"})
}
