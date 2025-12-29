package ConnManager

// 这是neighbor slot模块，负责管理与单个邻居节点的连接和通信
import (
	"sync"
	"time"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type NeighborSlot struct {
	neighborID string
	stream     pb.GossipChunkService_PushChunksClient
	cancel     func()

	mu        sync.Mutex
	lastAlive time.Time
}

func NewNeighborSlot(
	id string,
	stream pb.GossipChunkService_PushChunksClient,
	cancel func(),
) *NeighborSlot {
	return &NeighborSlot{
		neighborID: id,
		stream:     stream,
		cancel:     cancel,
		lastAlive:  time.Now(),
	}
}

func (s *NeighborSlot) SendChunk(chunk *pb.GossipChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.stream.Send(chunk)
	if err == nil {
		s.lastAlive = time.Now()
	}
	return err
}

func (s *NeighborSlot) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// 这里只做状态维护，不一定真的发包
			if time.Since(s.lastAlive) > 30*time.Second {
				s.cancel()
				return
			}
		}
	}()
}
