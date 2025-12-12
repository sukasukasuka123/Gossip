package MessageManage

import (
	sm "github.com/sukasukasuka123/Gossip/StorageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// T 是消息体 (Payload)
type GossipMessage[T any] struct {
	Hash     string `json:"hash"`      // 消息的哈希值
	FromHash string `json:"from_hash"` // 消息来自哪个节点
	Payload  T      `json:"payload"`
}

// MessageManager 是消息管理器
type MessageManager struct {
	MesSendChan  chan *pb.GossipMessage
	MesRecvChan  chan *pb.GossipMessage
	AckSendChan  chan *pb.GossipACK
	AckRecvChan  chan *pb.GossipACK
	CompleteChan chan string // 用于通知消息已完成 (Hash)
	SM           *sm.StorageManage
	NodeHash     string
}

// NewMessageManager 创建 MessageManager 实例
func NewMessageManager(nodeHash string, smgr *sm.StorageManage) *MessageManager {
	MM := &MessageManager{
		MesSendChan:  make(chan *pb.GossipMessage, 100),
		MesRecvChan:  make(chan *pb.GossipMessage, 100),
		AckSendChan:  make(chan *pb.GossipACK, 100),
		AckRecvChan:  make(chan *pb.GossipACK, 100),
		CompleteChan: make(chan string, 100),
		SM:           smgr,
		NodeHash:     nodeHash,
	}
	go MM.MessageManageLoop()
	return MM
}
