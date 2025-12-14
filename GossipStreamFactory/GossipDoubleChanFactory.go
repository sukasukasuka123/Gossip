package GossipStreamFactory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// DoubleStreamSlot 管理一个邻居的双流
type DoubleStreamSlot struct {
	MessageStream pb.Gossip_MessageStreamClient
	AckStream     pb.Gossip_AckStreamClient
	conn          *grpc.ClientConn

	lastUsed atomic.Int64
	closed   atomic.Bool
}

func (s *DoubleStreamSlot) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	// 优雅关闭：流在前，连接在后
	if s.MessageStream != nil {
		_ = s.MessageStream.CloseSend()
	}
	if s.AckStream != nil {
		_ = s.AckStream.CloseSend()
	}

	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *DoubleStreamSlot) IsClosed() bool {
	return s.closed.Load()
}

func (s *DoubleStreamSlot) Touch() {
	s.lastUsed.Store(time.Now().UnixNano())
}

type DoubleStreamFactory struct {
	slots sync.Map // nodeHash → *DoubleStreamSlot

	ttl    time.Duration
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	dialOpts []grpc.DialOption
}

func NewDoubleStreamFactory(ttl time.Duration, opts ...grpc.DialOption) *DoubleStreamFactory {
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

	f := &DoubleStreamFactory{
		ttl:      ttl,
		ctx:      ctx,
		cancel:   cancel,
		dialOpts: opts,
	}

	f.wg.Add(1)
	go f.gcLoop()

	return f
}

func (f *DoubleStreamFactory) gcLoop() {
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
				slot := value.(*DoubleStreamSlot)
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

func (f *DoubleStreamFactory) GetDoubleStream(nodeHash, endpoint string) (*DoubleStreamSlot, error) {
	// 快速路径：已有可用连接
	if v, ok := f.slots.Load(nodeHash); ok {
		slot := v.(*DoubleStreamSlot)
		if !slot.IsClosed() {
			slot.Touch()
			return slot, nil
		}
		f.slots.Delete(nodeHash)
	}

	// 慢速路径：建立新连接
	ctx, cancel := context.WithTimeout(f.ctx, 4*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint, f.dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s failed: %w", endpoint, err)
	}

	client := pb.NewGossipClient(conn)

	MessageStream, err := client.MessageStream(f.ctx)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("MessageStream failed: %w", err)
	}

	ackStream, err := client.AckStream(f.ctx)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("AckStream failed: %w", err)
	}

	slot := &DoubleStreamSlot{
		MessageStream: MessageStream,
		AckStream:     ackStream,
		conn:          conn,
	}
	slot.Touch()

	// 原子存储
	actual, loaded := f.slots.LoadOrStore(nodeHash, slot)
	if loaded {
		slot.Close()
		return actual.(*DoubleStreamSlot), nil
	}

	return slot, nil
}

func (f *DoubleStreamFactory) Remove(nodeHash string) {
	if v, ok := f.slots.LoadAndDelete(nodeHash); ok {
		v.(*DoubleStreamSlot).Close()
	}
}

func (f *DoubleStreamFactory) CloseAll() {
	f.cancel()
	f.wg.Wait()

	f.slots.Range(func(k, v any) bool {
		v.(*DoubleStreamSlot).Close()
		f.slots.Delete(k)
		return true
	})
}
