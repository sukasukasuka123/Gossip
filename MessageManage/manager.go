package MessageManage

import (
	"log"

	sm "github.com/sukasukasuka123/Gossip/StorageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// MessageManageLoop 消息调度循环
func (mm *MessageManager) MessageManageLoop() {
	for {
		select {
		case mes := <-mm.MesRecvChan:
			if err := mm.OnMessageRecv(mes); err != nil {
				log.Printf("[MessageManage-ERROR] OnMessageRecv: %v", err)
			}

		case mes := <-mm.MesSendChan:
			if err := mm.OnMessageSend(mes); err != nil {
				log.Printf("[MessageManage-ERROR] OnMessageSend: %v", err)
			}

		case ack := <-mm.AckRecvChan:
			if err := mm.OnAckRecv(ack); err != nil {
				log.Printf("[MessageManage-ERROR] OnAckRecv: %v", err)
			}

		case ack := <-mm.AckSendChan:
			if err := mm.OnAckSend(ack); err != nil {
				log.Printf("[MessageManage-ERROR] OnAckSend: %v", err)
			}
		}
	}
}

// OnMessageRecv 接收到消息
func (mm *MessageManager) OnMessageRecv(mes *pb.GossipMessage) error {
	// 使用 StorageManage 调度 InitState
	payload := sm.Payload{
		Hash:      mes.Hash,
		Neighbors: nil, // TODO: 这里可以传入节点邻居列表
	}
	mm.Storage.ScheduleWrite(sm.OpInitState, payload)
	return nil
}

// OnMessageSend 消息发送
func (mm *MessageManager) OnMessageSend(mes *pb.GossipMessage) error {
	payload := sm.Payload{
		Hash:     mes.Hash,
		NodeHash: mes.FromHash,
	}
	mm.Storage.ScheduleWrite(sm.OpMarkSent, payload)
	return nil
}

// OnAckRecv 收到 ACK
func (mm *MessageManager) OnAckRecv(ack *pb.GossipACK) error {
	payload := sm.Payload{
		Hash:     ack.Hash,
		NodeHash: ack.FromHash,
	}
	mm.Storage.ScheduleWrite(sm.OpUpdateState, payload)
	return nil
}

// OnAckSend 发送 ACK
func (mm *MessageManager) OnAckSend(ack *pb.GossipACK) error {
	// 如果只是调度存储更新，可选择调用 MarkSent 或 UpdateState
	payload := sm.Payload{
		Hash:     ack.Hash,
		NodeHash: ack.FromHash,
	}
	mm.Storage.ScheduleWrite(sm.OpMarkSent, payload)
	return nil
}
