package NodeManage

import (
	message "Gossip/MessageManage"
	"fmt" // 引入 fmt 用于日志格式化
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 假设 n.Logger 已在 GossipNode 结构体中定义
// 假设 n.NodeHash 用于指定写入的文件名

func (n *GossipNode[T]) RegisterRoutes(r *gin.Engine) {
	r.POST("/RecieveMessage", n.HandleReceiveMessage)
	r.POST("/RecieveAck", n.HandleReceiveAck)
}

// ==========================
// Handle Receive Message,接收实际消息的处理
// ==========================
func (n *GossipNode[T]) HandleReceiveMessage(c *gin.Context) {
	var msg message.GossipMessage[T]
	if err := c.ShouldBindJSON(&msg); err != nil {
		// 记录 JSON 绑定错误
		logText := fmt.Sprintf("[WARN] JSON Bind Error: %s", err.Error())
		n.Logger.Log(logText, n.NodeHash)

		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. 原子地初始化状态
	isNew, err := n.Storage.InitState(msg.Hash, keys(n.Neighbors))
	if err != nil {
		// 记录存储初始化错误
		logText := fmt.Sprintf("[ERROR] Storage InitState failed for hash %s: %v", msg.Hash, err)
		n.Logger.Log(logText, n.NodeHash)

		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	// 2. 立即回复 ACK (无论是不是新消息)
	if msg.FromHash != "" && msg.FromHash != n.NodeHash {
		go n.sendAckToSenders(msg.Hash, msg.FromHash)
	}

	// 3. 如果不是新消息，直接返回
	if !isNew {
		// 记录重复消息
		logText := fmt.Sprintf("[DEBUG] Repeated message received: %s (from %s). Sending ACK only.", msg.Hash, msg.FromHash)
		n.Logger.Log(logText, n.NodeHash)

		c.JSON(http.StatusOK, gin.H{"status": "repeated"})
		return
	}

	// 记录接收到新消息
	logText := fmt.Sprintf("[INFO] NEW MESSAGE RECEIVED and accepted: %s (from %s)", msg.Hash, msg.FromHash)
	n.Logger.Log(logText, n.NodeHash)

	// 4. 异步广播 (只广播新消息)
	go n.broadcastToTargets(msg)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ==========================
// Handle Receive ACK，接收 ACK 的处理
// ==========================
func (n *GossipNode[T]) HandleReceiveAck(c *gin.Context) {
	var ack message.GossipAck
	if err := c.ShouldBindJSON(&ack); err != nil {
		// 记录 ACK JSON 绑定错误
		logText := fmt.Sprintf("[WARN] ACK JSON Bind Error: %s", err.Error())
		n.Logger.Log(logText, n.NodeHash)

		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 记录收到 ACK
	logText := fmt.Sprintf("[DEBUG] Received ACK for hash %s from %s", ack.Hash, ack.FromHash)
	n.Logger.Log(logText, n.NodeHash)

	n.Storage.UpdateState(ack.Hash, ack.FromHash)

	state := n.Storage.GetStates(ack.Hash)

	// 检查状态是否为 nil（可能已被并发删除）
	if state == nil {
		logText := fmt.Sprintf("[WARN] State for hash %s is nil after ACK from %s. State was likely deleted concurrently.", ack.Hash, ack.FromHash)
		n.Logger.Log(logText, n.NodeHash)
		c.JSON(http.StatusOK, gin.H{"status": "ack received, state nil"})
		return
	}

	allOK := true
	for _, v := range state {
		if !v {
			allOK = false
			break
		}
	}

	if allOK {
		go func(hash string, neighborsCount int) {
			// 延迟时间: 邻居数 * 1秒
			delay := time.Duration(neighborsCount) * time.Second

			// 记录延迟信息
			delayLog := fmt.Sprintf("[INFO] Deletion for hash %s scheduled in %d seconds.", hash, neighborsCount)
			n.Logger.Log(delayLog, n.NodeHash)

			time.Sleep(delay)

			// 执行删除
			n.Storage.DeleteState(hash)

			// 记录延迟删除成功
			deleteLog := fmt.Sprintf("[INFO] All ACKs received for hash %s. State successfully deleted after delay.", hash)
			n.Logger.Log(deleteLog, n.NodeHash)

		}(ack.Hash, len(n.Neighbors)) // 将哈希和邻居数传入协程
	}

	c.JSON(http.StatusOK, gin.H{"status": "ack received"})
}
