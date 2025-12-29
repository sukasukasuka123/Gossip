package ConnManager

// 这是neighbor slot模块，负责管理与单个邻居节点的连接和通信
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
}

func NewNeighborSlot(
	id string,
	stream pb.GossipChunkService_PushChunksClient,
	windowSize int,
	cancel func(),
) *NeighborSlot {
	slidingWindow := *SlidingWindow.NewSlidingWindowManager[*pb.GossipChunk](windowSize)
	return &NeighborSlot{
		neighborID:    id,
		stream:        stream,
		cancel:        cancel,
		slidingWindow: &slidingWindow,
		lastAlive:     time.Now(),
	}
}

func (s *NeighborSlot) ReadySendMsg(
	ctx context.Context,
	chunks []*pb.GossipChunk,
) {
	// 1. 把所有 chunk 推入滑动窗口的缓存
	for _, chunk := range chunks {
		// 这里用 PayloadHash + ChunkIndex 作为唯一 key
		key := chunk.PayloadHash + ":" + fmt.Sprint(chunk.ChunkIndex)
		s.slidingWindow.PushResource(key, chunk)
	}

	// 2. 启动滑动窗口资源调度
	go s.slidingWindow.ResourceManage(ctx, func(chunk *pb.GossipChunk) {
		// 真正的出队处理：发包
		err := s.SendChunk(chunk)
		if err != nil {
			// 这里先不复杂化：失败直接断链
			// 之后可以演进为重试 / 退回缓存 / 降级
			s.cancel()
			return
		}
	})
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
