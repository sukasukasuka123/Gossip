package GossipStreamFactory

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
	"google.golang.org/grpc"
)

type StreamSlot struct {
	stream   pb.Gossip_StreamClient
	conn     *grpc.ClientConn
	lastUsed int64 // UnixNano，使用原子操作更新
}

type StreamFactory struct {
	streams sync.Map // key: nodeHash, value: *StreamSlot
	ttl     time.Duration
}

func NewStreamFactory(ttl time.Duration) *StreamFactory {
	sf := &StreamFactory{
		ttl: ttl,
	}
	go sf.gcLoop()
	return sf
}

func (sf *StreamFactory) gcLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().UnixNano()
		sf.streams.Range(func(key, value any) bool {
			slot := value.(*StreamSlot)
			last := atomic.LoadInt64(&slot.lastUsed)
			if now-last > sf.ttl.Nanoseconds() {
				if slot.conn != nil {
					_ = slot.conn.Close()
				}
				sf.streams.Delete(key)
			}
			return true
		})
	}
}

// 确保 stream 存在，不存在则建立
func (sf *StreamFactory) GetStream(nodeHash, endpoint string) (pb.Gossip_StreamClient, error) {
	now := time.Now().UnixNano()

	if value, ok := sf.streams.Load(nodeHash); ok {
		slot := value.(*StreamSlot)
		atomic.StoreInt64(&slot.lastUsed, now)
		return slot.stream, nil
	}

	// 没有找到，则建立新的连接
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client := pb.NewGossipClient(conn)
	stream, err := client.Stream(context.Background())
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	slot := &StreamSlot{
		stream:   stream,
		conn:     conn,
		lastUsed: now,
	}

	// 尝试存入 map，避免并发重复创建
	actual, loaded := sf.streams.LoadOrStore(nodeHash, slot)
	if loaded {
		// 其他协程已经创建了，关闭自己创建的连接
		_ = conn.Close()
		return actual.(*StreamSlot).stream, nil
	}

	return stream, nil
}

func (sf *StreamFactory) CloseAllStreams() {
	sf.streams.Range(func(key, value any) bool {
		slot := value.(*StreamSlot)
		if slot.conn != nil {
			_ = slot.conn.Close()
		}
		sf.streams.Delete(key)
		return true
	})
}
