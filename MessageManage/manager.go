package MessageManage

import (
	"log"

	sm "github.com/sukasukasuka123/Gossip/StorageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// 消息调度循环
func (mm *MessageManager) MessageManageLoop() {
	for {
		select {
		case mes := <-mm.MesRecvChan:
			if mes == nil { // <-- 新增检查
				//log.Print("[MM-WARN] Received nil message on MesRecvChan, skipping.")
				continue
			}
			if err := mm.OnMessageRecv(mes); err != nil {
				log.Printf("[MM-ERROR] OnMessageRecv: %v", err)
			}

		case mes := <-mm.MesSendChan:
			if err := mm.OnMessageSend(mes); err != nil {
				log.Printf("[MM-ERROR] OnMessageSend: %v", err)
			}

		case ack := <-mm.AckRecvChan:
			if ack == nil { // <-- 新增检查
				//log.Print("[MM-WARN] Received nil ack on AckRecvChan, skipping.")
				continue
			}
			if err := mm.OnAckRecv(ack); err != nil {
				log.Printf("[MM-ERROR] OnAckRecv: %v", err)
			}

		case ack := <-mm.AckSendChan:
			if err := mm.OnAckSend(ack); err != nil {
				log.Printf("[MM-ERROR] OnAckSend: %v", err)
			}
		}
	}
}

// ==== 四个事件的处理 ====

func (mm *MessageManager) OnMessageRecv(mes *pb.GossipMessage) error {
	log.Print("OnMessageRecv called in MessageManager, FromHash: " + mes.GetFromHash())
	mm.SM.Schedule(sm.OpInitState, sm.Payload{
		Hash:      mes.Hash,
		Neighbors: nil, // 可选：初始化邻居列表
	})
	ack := &pb.GossipACK{
		Hash:     mes.Hash,
		FromHash: mm.NodeHash,
	}
	select {
	case mm.AckSendChan <- ack:
		// 成功放入 ACK 队列
	default:
		// 如果 ACK 队列满，说明 gRPC 发送太慢，记录警告，但不阻塞消息处理
		log.Printf("[MM-WARN] AckSendChan full, dropping ACK for %s. (Gossip is congested)", mes.Hash)
	}
	return nil
}

func (mm *MessageManager) OnMessageSend(mes *pb.GossipMessage) error {
	mm.SM.Schedule(sm.OpMarkSent, sm.Payload{
		Hash:     mes.Hash,
		NodeHash: mes.FromHash,
	})
	return nil
}

func (mm *MessageManager) OnAckRecv(ack *pb.GossipACK) error {
	log.Print("[" + mm.NodeHash + "]OnAckRecv called in MessageManager, FromHash: " + ack.GetFromHash())
	log.Print("before schedule")
	mm.SM.Schedule(sm.OpUpdateState, sm.Payload{
		Hash:     ack.Hash,
		NodeHash: ack.FromHash,
	})
	log.Print("after schedule")
	// 2. [关键修改] 使用 select 语句发送完成通知，确保非阻塞
	select {
	case mm.CompleteChan <- ack.Hash:
		log.Printf("[MM] Sent ACK Complete signal for %s", ack.Hash)
	default:
		// 如果 CompleteChan 没有消费者或者已满，则跳过发送，避免阻塞。
		// 在生产环境中，这里可能需要记录警告日志。
		log.Printf("[MM-WARN] CompleteChan full or unread, skipped signal for %s", ack.Hash)
	}

	return nil
}

func (mm *MessageManager) OnAckSend(ack *pb.GossipACK) error {
	mm.SM.Schedule(sm.OpMarkSent, sm.Payload{
		Hash:     ack.Hash,
		NodeHash: ack.FromHash,
	})
	select {
	case mm.CompleteChan <- ack.Hash:
		// 通知发送成功
	default:
		// 如果 CompleteChan 没有消费者，避免阻塞 OnAckRecv
	}
	return nil
}
