package Storage

import "time"

type TTLEvent struct {
	Type TTLEventType
	Data interface{}
}

type TTLEventType int

const (
	EventNeighborAdded TTLEventType = iota
	EventAckReceived
	EventTickDecay // 定期衰减
)

func (s *LocalStorage) ttlLoop() {

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {

		// 定期衰减
		case <-ticker.C:
			s.ttlChan <- TTLEvent{Type: EventTickDecay}

		// 处理事件
		case ev := <-s.ttlChan:
			switch ev.Type {

			// 节点新增 → limit 指数增加
			case EventNeighborAdded:
				old := s.shortLimit.Load()
				newLimit := old * 2
				if newLimit > 64 {
					newLimit = 64
				}
				s.shortLimit.Store(newLimit)

			// 收到 ACK → 当前 TTL 向 limit 靠近
			case EventAckReceived:
				limit := s.shortLimit.Load()
				cur := s.shortTTL.Load()

				// 指数趋近 limit
				if cur < limit {
					increase := (limit - cur) / 2
					if increase < 1 {
						increase = 1
					}
					newTTL := cur + increase
					if newTTL > limit {
						newTTL = limit
					}
					s.shortTTL.Store(newTTL)
				}

			// 定时衰减 → 当前 TTL 缓慢回落
			case EventTickDecay:
				cur := s.shortTTL.Load()
				if cur > 2 { // 不低于2秒
					s.shortTTL.Store(cur - 1)
				}
			}
		}
	}
}

func (s *LocalStorage) RecieveAck() {
	s.ttlChan <- TTLEvent{Type: EventAckReceived}
}
