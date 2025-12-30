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

	ctx    context.Context
	cancel context.CancelFunc

	lastAliveMu sync.Mutex
	lastAlive   time.Time

	window   *SlidingWindow.SlidingWindowManager[*pb.GossipChunk]
	inFlight sync.Map // key -> struct{}
}

func NewNeighborSlot(
	id string,
	stream pb.GossipChunkService_PushChunksClient,
	windowSize int,
	parentCtx context.Context,
) *NeighborSlot {
	ctx, cancel := context.WithCancel(parentCtx)

	s := &NeighborSlot{
		neighborID: id,
		stream:     stream,
		ctx:        ctx,
		cancel:     cancel,
		window:     SlidingWindow.NewSlidingWindowManager[*pb.GossipChunk](windowSize),
		lastAlive:  time.Now(),
	}

	go s.runSlidingWindow()
	return s
}

func (s *NeighborSlot) runSlidingWindow() {
	s.window.ResourceManageBatch(s.ctx, func(batch map[string]*pb.GossipChunk) {
		for key, chunk := range batch {
			if err := s.stream.Send(chunk); err != nil {
				s.cancel()
				return
			}
			s.inFlight.Store(key, struct{}{})
			s.touchAlive()
		}
	}, 8) // 每次批量 8 条
}

func (s *NeighborSlot) HandleAckBatch(acks []*pb.GossipChunkAck) {
	for _, ack := range acks {
		key := fmt.Sprintf("%s:%d", ack.PayloadHash, ack.ChunkIndex)
		if _, ok := s.inFlight.LoadAndDelete(key); ok {
			s.window.ReleaseBatch(1) // 或 ReleaseBatch(1)
		}
	}
}

func (s *NeighborSlot) ReadySendMsg(chunks []*pb.GossipChunk) {
	for _, chunk := range chunks {
		key := fmt.Sprintf("%s:%d", chunk.PayloadHash, chunk.ChunkIndex)
		s.window.PushResource(key, chunk)
	}
}

func (s *NeighborSlot) HandleAck(ack *pb.GossipChunkAck) {
	key := fmt.Sprintf("%s:%d", ack.PayloadHash, ack.ChunkIndex)

	if _, ok := s.inFlight.LoadAndDelete(key); ok {
		s.window.ReleaseBatch(1)
	}
}

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

func (s *NeighborSlot) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.lastAliveMu.Lock()
				last := s.lastAlive
				s.lastAliveMu.Unlock()

				if time.Since(last) > 30*time.Second {
					s.cancel()
					return
				}
			}
		}
	}()
}

func (s *NeighborSlot) touchAlive() {
	s.lastAliveMu.Lock()
	s.lastAlive = time.Now()
	s.lastAliveMu.Unlock()
}

func (s *NeighborSlot) Close() error {
	s.cancel()
	return s.stream.CloseSend()
}
