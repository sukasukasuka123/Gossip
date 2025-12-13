package MessageManage

import (
	"fmt"
	"log"

	sm "github.com/sukasukasuka123/Gossip/StorageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// 消息调度循环
func (mm *MessageManager) MessageManageLoop() {
	for {
		select {
		case mes := <-mm.MesRecvChan:
			if mes == nil {
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
			if ack == nil {
				continue
			}
			if err := mm.OnAckRecv(ack); err != nil {
				log.Printf("[MM-ERROR] OnAckRecv: %v", err)
			}
			log.Printf("[MM] OnAckRecv processed ACK for %s from %s", ack.Hash, ack.FromHash)

			// ❌ 移除这个 case！AckSendChan 由 Node 层的 globalAckRouter 处理
			// case ack := <-mm.AckSendChan:
			//     if err := mm.OnAckSend(ack); err != nil {
			//         log.Printf("[MM-ERROR] OnAckSend: %v", err)
			//     }
		}
	}
}

// ==== 四个事件的处理 ====

// OnMessageRecv 当收到消息时：记录来源，初始化状态，准备发送ACK
func (mm *MessageManager) OnMessageRecv(mes *pb.GossipMessage) error {
	log.Printf("[MM] OnMessageRecv called, Hash:%s FromHash:%s", mes.Hash, mes.FromHash)

	// 关键：记录这个消息来自哪个节点
	mm.messageSourceMap.Store(mes.Hash, mes.FromHash)

	mm.SM.Storage.InitState(mes.Hash, []string{mes.FromHash})

	// 创建ACK并放入发送队列
	ack := &pb.GossipACK{
		Hash:     mes.Hash,
		FromHash: mm.NodeHash, // ACK的FromHash是本节点
	}

	select {
	case mm.AckSendChan <- ack:
		log.Printf("[MM] ACK queued for sending: Hash=%s", ack.Hash)
	default:
		// 异步发送，避免阻塞
		go func(a *pb.GossipACK) {
			mm.AckSendChan <- a
		}(ack)
	}
	return nil
}

// OnMessageSend 当发送消息时：标记已发送
func (mm *MessageManager) OnMessageSend(mes *pb.GossipMessage) error {
	mm.SM.Schedule(sm.OpMarkSent, sm.Payload{
		Hash:     mes.Hash,
		NodeHash: mes.FromHash,
	})
	return nil
}

// OnAckRecv 当收到ACK时：更新状态，通知完成
func (mm *MessageManager) OnAckRecv(ack *pb.GossipACK) error {
	if err := mm.SM.Storage.UpdateState(ack.Hash, ack.FromHash); err != nil {
		return err
	}

	select {
	case mm.CompleteChan <- ack.Hash:
		fmt.Printf("[MM] %s received ACK for %s\n", mm.NodeHash, ack.Hash)
	default:
		log.Printf("[MM-WARN] CompleteChan full, skipped signal for %s", ack.Hash)
	}

	return nil
}

// OnAckSend 当ACK被实际发送后调用：标记已发送，通知完成
func (mm *MessageManager) OnAckSend(ack *pb.GossipACK) error {
	mm.SM.Schedule(sm.OpMarkSent, sm.Payload{
		Hash:     ack.Hash,
		NodeHash: ack.FromHash,
	})

	select {
	case mm.CompleteChan <- ack.Hash:
		log.Printf("[MM] ACK send completed for %s", ack.Hash)
	default:
		// 避免阻塞
	}
	return nil
}

// GetMessageSource 获取消息的来源节点
func (mm *MessageManager) GetMessageSource(messageHash string) (string, bool) {
	val, ok := mm.messageSourceMap.Load(messageHash)
	if !ok {
		return "", false
	}
	return val.(string), true
}

// CleanupMessageSource 清理消息来源记录（可选，用于内存管理）
func (mm *MessageManager) CleanupMessageSource(messageHash string) {
	mm.messageSourceMap.Delete(messageHash)
}
