// ConnManager/neighbor_slot.go
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
	ctx        context.Context
	cancel     context.CancelFunc

	mu        sync.Mutex
	lastAlive time.Time

	slidingWindow *SlidingWindow.SlidingWindowManager[*pb.GossipChunk]
	inFlight      map[string]*pb.GossipChunk
}

func NewNeighborSlot(
	id string,
	stream pb.GossipChunkService_PushChunksClient,
	windowSize int,
	parentCtx context.Context,
) *NeighborSlot {
	ctx, cancel := context.WithCancel(parentCtx)

	s := &NeighborSlot{
		neighborID:    id,
		stream:        stream,
		ctx:           ctx,
		cancel:        cancel,
		slidingWindow: SlidingWindow.NewSlidingWindowManager[*pb.GossipChunk](windowSize),
		inFlight:      make(map[string]*pb.GossipChunk),
		lastAlive:     time.Now(),
	}

	// ✅ 在初始化时启动唯一的滑动窗口管理协程
	go s.runSlidingWindow()

	return s
}

// runSlidingWindow 运行滑动窗口，这个 slot 只会有一个这样的协程
func (s *NeighborSlot) runSlidingWindow() {
	s.slidingWindow.ResourceManage(s.ctx, func(key string, chunk *pb.GossipChunk) {
		// 记录 in-flight
		s.mu.Lock()
		s.inFlight[key] = chunk
		s.mu.Unlock()

		// 发送 chunk
		if err := s.SendChunk(chunk); err != nil {
			// 发送失败，关闭整个 slot
			s.cancel()
		}
	})
}

// ReadySendMsg 只负责将消息的所有 chunks 推入滑动窗口
// ✅ 不再创建新的协程，只是推送数据
func (s *NeighborSlot) ReadySendMsg(chunks []*pb.GossipChunk) {
	for _, chunk := range chunks {
		key := fmt.Sprintf("%s:%d", chunk.PayloadHash, chunk.ChunkIndex)
		s.slidingWindow.PushResource(key, chunk)
	}
}

// HandleAck 收到 ACK 时释放 in-flight 并释放窗口 slot
func (s *NeighborSlot) HandleAck(ack *pb.GossipChunkAck) {
	key := fmt.Sprintf("%s:%d", ack.PayloadHash, ack.ChunkIndex)

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
func (s *NeighborSlot) StartRecvAck() {
	go func() {
		for {
			select {
			case <-s.ctx.Done():
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

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				lastAlive := s.lastAlive
				s.mu.Unlock()

				if time.Since(lastAlive) > 30*time.Second {
					s.cancel()
					return
				}
			}
		}
	}()
}

// Close 优雅关闭
func (s *NeighborSlot) Close() error {
	s.cancel()
	return s.stream.CloseSend()
}
