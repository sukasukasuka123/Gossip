package NeighborManage

import (
	"io"
	"log"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
	MM "github.com/sukasukasuka123/Gossip/MessageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// NeighborSlot 管理一个邻居的消息和 ack 流
type NeighborSlot struct {
	PeerHash string
	Slot     *GossipStreamFactory.DoubleStreamSlot

	MsgChan chan *pb.GossipMessage
	AckChan chan *pb.GossipACK

	closed chan struct{}
}

// NewNeighborSlot 创建并启动 recv loop
func NewNeighborSlot(peerHash string, slot *GossipStreamFactory.DoubleStreamSlot) *NeighborSlot {
	ns := &NeighborSlot{
		PeerHash: peerHash,
		Slot:     slot,
		MsgChan:  make(chan *pb.GossipMessage, 100),
		AckChan:  make(chan *pb.GossipACK, 100),
		closed:   make(chan struct{}),
	}

	go ns.recvMessageLoop()
	go ns.recvAckLoop()
	return ns
}

// Close 关闭流
func (ns *NeighborSlot) Close() {
	select {
	case <-ns.closed:
		return
	default:
		close(ns.closed)
		ns.Slot.Close()
	}
}

// recvMessageLoop 从 MessageStream 接收消息
func (ns *NeighborSlot) recvMessageLoop() {
	for {
		select {
		case <-ns.closed:
			return
		default:
		}
		msg, err := ns.Slot.MessageStream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("[NeighborSlot %s] MessageStream recv EOF", ns.PeerHash)
			} else {
				log.Printf("[NeighborSlot %s] MessageStream recv err: %v", ns.PeerHash, err)
			}
			ns.Close()
			return
		}
		ack := &pb.GossipACK{
			Hash:     msg.Hash,
			FromHash: ns.PeerHash,
		}
		err1 := ns.SendAck(ack)
		if err1 != nil {
			log.Printf("[NeighborSlot %s] SendAck err: %v", ns.PeerHash, err1)
		}
		ns.MsgChan <- msg
	}
}

// recvAckLoop 从 AckStream 接收 ack
func (ns *NeighborSlot) recvAckLoop() {
	for {
		select {
		case <-ns.closed:
			return
		default:
		}

		ack, err := ns.Slot.AckStream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("[NeighborSlot %s] AckStream recv EOF", ns.PeerHash)
			} else {
				log.Printf("[NeighborSlot %s] AckStream recv err: %v", ns.PeerHash, err)
			}
			ns.Close()
			return
		}
		ns.AckChan <- ack
	}
}

// SendMessage 发送消息（线程安全）
func (ns *NeighborSlot) SendMessage(msg *pb.GossipMessage) error {
	ns.Slot.Touch()
	if msg == nil {
		log.Printf("[NeighborSlot %s] SendMessage called with nil message, skipping.", ns.PeerHash)
		return nil // 直接返回成功或特定的错误
	}
	log.Print("send message called from NeighborSlot to " + ns.PeerHash + ", FromHash: " + msg.FromHash)
	return ns.Slot.MessageStream.Send(msg)
}

// SendAck 发送 ack（线程安全）
func (ns *NeighborSlot) SendAck(ack *pb.GossipACK) error {
	ns.Slot.Touch()
	if ack == nil {
		log.Printf("[NeighborSlot %s] SendAck called with nil ACK, skipping.", ns.PeerHash)
		return nil // 直接返回成功或特定的错误
	}
	return ns.Slot.AckStream.Send(ack)
}
func (ns *NeighborSlot) ForwardTo(mm *MM.MessageManager) {
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
