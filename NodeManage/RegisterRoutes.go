package NodeManage

import (
	message "Gossip/MessageManage"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (n *GossipNode[T]) RegisterRoutes(r *gin.Engine) {
	r.POST("/RecieveMessage", n.HandleReceiveMessage)
	r.POST("/RecieveAck", n.HandleReceiveAck)
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
