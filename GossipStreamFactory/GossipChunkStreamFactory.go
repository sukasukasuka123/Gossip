package GossipStreamFactory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type GossipChunkStreamSlot struct {
	MsgStream pb.GossipChunkService_ChunkMessageStreamClient
	AckStream pb.GossipChunkService_ChunkAckStreamClient
	conn      *grpc.ClientConn

	lastUsed atomic.Int64
	closed   atomic.Bool
}

func (s *GossipChunkStreamSlot) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	// 优雅关闭：流在前，连接在后
	if s.MsgStream != nil {
		_ = s.MsgStream.CloseSend()
	}
	if s.AckStream != nil {
		_ = s.AckStream.CloseSend()
	}

	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *GossipChunkStreamSlot) IsClosed() bool {
	return s.closed.Load()
}

func (s *GossipChunkStreamSlot) Touch() {
	s.lastUsed.Store(time.Now().UnixNano())
}

type GossipChunkStreamFactory struct {
	slots sync.Map // nodeHash → *GossipChunkStreamSlot

	ttl    time.Duration
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	dialOpts []grpc.DialOption
}

func NewGossipChunkStreamFactory(ttl time.Duration, opts ...grpc.DialOption) *GossipChunkStreamFactory {
	ctx, cancel := context.WithCancel(context.Background())

	if len(opts) == 0 {
		opts = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                60 * time.Second,
				Timeout:             20 * time.Second,
				PermitWithoutStream: false,
			}),
		}
	}

	f := &GossipChunkStreamFactory{
		ttl:      ttl,
		ctx:      ctx,
		cancel:   cancel,
		dialOpts: opts,
	}

	f.wg.Add(1)
	go f.gcLoop()

	return f
}
func (f *GossipChunkStreamFactory) gcLoop() {
	defer f.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			f.slots.Range(func(key, value any) bool {
				slot := value.(*GossipChunkStreamSlot)
				last := time.Unix(0, slot.lastUsed.Load())

				if now.Sub(last) > f.ttl {
					slot.Close()
					f.slots.Delete(key)
				}
				return true
			})
		}
	}
}
func (f *GossipChunkStreamFactory) GetStream(nodeHash, endpoint string) (*GossipChunkStreamSlot, error) {
	// 快速路径
	if v, ok := f.slots.Load(nodeHash); ok {
		slot := v.(*GossipChunkStreamSlot)
		if !slot.IsClosed() {
			slot.Touch()
			return slot, nil
		}
		f.slots.Delete(nodeHash)
	}

	// 慢速路径：新建连接
	ctx, cancel := context.WithTimeout(f.ctx, 4*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, f.dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s failed: %w", endpoint, err)
	}

	client := pb.NewGossipChunkServiceClient(conn)

	msgStream, err := client.ChunkMessageStream(f.ctx)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ChunkMessageStream failed: %w", err)
	}

	ackStream, err := client.ChunkAckStream(f.ctx)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ChunkAckStream failed: %w", err)
	}

	slot := &GossipChunkStreamSlot{
		MsgStream: msgStream,
		AckStream: ackStream,
		conn:      conn,
	}
	slot.Touch()

	actual, loaded := f.slots.LoadOrStore(nodeHash, slot)
	if loaded {
		slot.Close()
		return actual.(*GossipChunkStreamSlot), nil
	}

	return slot, nil
}
func (f *GossipChunkStreamFactory) Remove(nodeHash string) {
	if v, ok := f.slots.LoadAndDelete(nodeHash); ok {
		v.(*GossipChunkStreamSlot).Close()
	}
}

func (f *GossipChunkStreamFactory) CloseAll() {
	f.cancel()
	f.wg.Wait()

	f.slots.Range(func(k, v any) bool {
		v.(*GossipChunkStreamSlot).Close()
		f.slots.Delete(k)
		return true
	})
}
