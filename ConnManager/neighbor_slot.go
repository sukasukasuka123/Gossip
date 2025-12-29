package ConnManager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sukasukasuka123/Gossip/SlidingWindow"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

type NeighborSlot struct {
	neighborID string
	stream     pb.GossipChunkService_PushChunksClient
	cancel     func()

	mu        sync.Mutex
	lastAlive time.Time

	slidingWindow *SlidingWindow.SlidingWindowManager[*pb.GossipChunk]
	inFlight      map[string]*pb.GossipChunk
}

func NewNeighborSlot(
	id string,
	stream pb.GossipChunkService_PushChunksClient,
	windowSize int,
	cancel func(),
) *NeighborSlot {
	s := &NeighborSlot{
		neighborID:    id,
		stream:        stream,
		cancel:        cancel,
		slidingWindow: SlidingWindow.NewSlidingWindowManager[*pb.GossipChunk](windowSize),
		inFlight:      make(map[string]*pb.GossipChunk),
		lastAlive:     time.Now(),
	}
	return s
}

// ReadySendMsg 统一推入 neighbor 窗口
func (s *NeighborSlot) ReadySendMsg(ctx context.Context, chunks []*pb.GossipChunk) {
	for _, chunk := range chunks {
		key := chunk.PayloadHash + ":" + fmt.Sprint(chunk.ChunkIndex)
		s.slidingWindow.PushResource(key, chunk)
	}

	go s.slidingWindow.ResourceManage(ctx, func(key string, chunk *pb.GossipChunk) {
		s.mu.Lock()
		s.inFlight[key] = chunk
		s.mu.Unlock()

		if err := s.SendChunk(chunk); err != nil {
			s.cancel()
		}
	})
}

// HandleAck 收到 ACK 时释放 in-flight 并释放窗口 slot
func (s *NeighborSlot) HandleAck(ack *pb.GossipChunkAck) {
	key := ack.PayloadHash + ":" + fmt.Sprint(ack.ChunkIndex)

	s.mu.Lock()
	_, exists := s.inFlight[key]
	if exists {
		delete(s.inFlight, key)
	}
	s.mu.Unlock()

	if exists {
		s.slidingWindow.Release()
	}
}

// StartRecvAck 循环接收 ACK
func (s *NeighborSlot) StartRecvAck(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ack, err := s.stream.Recv()
			if err != nil {
				s.cancel()
				return
			}
			s.HandleAck(ack)
		}
	}()
}

// SendChunk 发送 chunk
func (s *NeighborSlot) SendChunk(chunk *pb.GossipChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.stream.Send(chunk)
	if err == nil {
		s.lastAlive = time.Now()
	}
	return err
}

// StartHeartbeat 心跳检查
func (s *NeighborSlot) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if time.Since(s.lastAlive) > 30*time.Second {
				s.cancel()
				return
			}
		}
	}()
}
