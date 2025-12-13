package NeighborManage

import (
	"log"
	"strings"
	"time"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
	MM "github.com/sukasukasuka123/Gossip/MessageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// NeighborSlot 管理一个邻居的消息和 ack 流
type NeighborSlot struct {
	PeerHash string
	Slot     *GossipStreamFactory.DoubleStreamSlot

	// 接收通道
	MsgChan chan *pb.GossipMessage
	AckChan chan *pb.GossipACK

	// 导出：专门用于写ACK的通道（Node层会写入此通道）
	AckWriteChan chan *pb.GossipACK

	closed chan struct{}
}

// NewNeighborSlot 创建邻居槽
func NewNeighborSlot(peerHash string, slot *GossipStreamFactory.DoubleStreamSlot) *NeighborSlot {
	ns := &NeighborSlot{
		PeerHash:     peerHash,
		Slot:         slot,
		MsgChan:      make(chan *pb.GossipMessage, 10000),
		AckChan:      make(chan *pb.GossipACK, 10000),
		AckWriteChan: make(chan *pb.GossipACK, 10000),
		closed:       make(chan struct{}),
	}

	return ns
}

// Close 关闭流
func (ns *NeighborSlot) Close() {
	select {
	case <-ns.closed:
		return
	default:
		// 先关闭 closed channel，通知所有 goroutine 停止
		close(ns.closed)

		// 给一点时间让 AckWriter 完成当前的发送
		time.Sleep(50 * time.Millisecond)

		// 关闭底层流
		ns.Slot.Close()

		log.Printf("[NeighborSlot %s] Closed", ns.PeerHash)
	}
}

// SendMessage 发送消息（线程安全）
func (ns *NeighborSlot) SendMessage(msg *pb.GossipMessage) error {
	ns.Slot.Touch()
	if msg == nil {
		log.Printf("[NeighborSlot %s] SendMessage called with nil message, skipping.", ns.PeerHash)
		return nil
	}
	log.Printf("[NeighborSlot] Send message to %s, Hash:%s FromHash:%s", ns.PeerHash, msg.Hash, msg.FromHash)
	return ns.Slot.MessageStream.Send(msg)
}

// SendAck 发送 ack（线程安全）
func (ns *NeighborSlot) SendAck(ack *pb.GossipACK) error {
	ns.Slot.Touch()
	if ack == nil {
		log.Printf("[NeighborSlot %s] SendAck called with nil ACK, skipping.", ns.PeerHash)
		return nil
	}
	log.Printf("[NeighborSlot %s] ACK_SEND From:%s Hash:%s", ns.PeerHash, ack.FromHash, ack.Hash)
	return ns.Slot.AckStream.Send(ack)
}

// ForwardTo 转发接收到的消息和ACK到MessageManager
func (ns *NeighborSlot) ForwardTo(mm *MM.MessageManager) {
	// 转发接收到的消息
	go func() {
		for {
			select {
			case msg := <-ns.MsgChan:
				mm.MesRecvChan <- msg
			case <-ns.closed:
				return
			}
		}
	}()

	// 转发接收到的ACK
	go func() {
		for {
			select {
			case ack := <-ns.AckChan:
				mm.AckRecvChan <- ack
			case <-ns.closed:
				return
			}
		}
	}()
}

// StartAckWriter 启动ACK写入协程
// 从 AckWriteChan 读取ACK，实际发送到网络，然后通知MM
func (ns *NeighborSlot) StartAckWriter(mm *MM.MessageManager) {
	go func() {
		for {
			select {
			case ack := <-ns.AckWriteChan:
				if ack == nil {
					continue
				}
				// 实际发送ACK到网络
				if err := ns.SendAck(ack); err != nil {
					// 检查是否是因为流已关闭
					if isStreamClosedError(err) {
						log.Printf("[NeighborSlot %s] Stream closed, stopping AckWriter", ns.PeerHash)
						return
					}
					log.Printf("[NeighborSlot %s] SendAck failed: %v", ns.PeerHash, err)
					// 发送失败，可能需要重试或者通知上层
					continue
				}
				// 发送成功，通知MessageManager更新状态
				if err := mm.OnAckSend(ack); err != nil {
					log.Printf("[NeighborSlot %s] OnAckSend failed: %v", ns.PeerHash, err)
				}
			case <-ns.closed:
				log.Printf("[NeighborSlot %s] AckWriter stopped", ns.PeerHash)
				return
			}
		}
	}()
}

// isStreamClosedError 检查错误是否是因为流已关闭
func isStreamClosedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "CloseSend") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "transport is closing")
}
