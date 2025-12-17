package test

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sukasukasuka123/Gossip/GossipStreamFactory"
	"github.com/sukasukasuka123/Gossip/Logger"
	"github.com/sukasukasuka123/Gossip/NeighborManage"
	"github.com/sukasukasuka123/Gossip/NodeManage"
	"github.com/sukasukasuka123/Gossip/Router"
	"github.com/sukasukasuka123/Gossip/Storage"
	"github.com/sukasukasuka123/Gossip/StorageManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

func init() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
}

// =======================
// Go 里的 static 等价物
// =======================

var globalNodeID int64 = 0

func nextNodeID() int64 {
	return atomic.AddInt64(&globalNodeID, 1)
}

// =======================
// 默认节点工厂函数
// =======================

// newDefaultNode
// - 不改你任何 Node 结构
// - 不隐藏端口（避免 benchmark 冲突）
// - 自动生成 nodeHash
func newDefaultNode(
	factory *GossipStreamFactory.DoubleStreamFactory,
	port string,
	storageSlots int64,
	storageTTL time.Duration,
) (*NodeManage.DoubleStreamNode, string) {

	id := nextNodeID()
	nodeHash := fmt.Sprintf("node-%d", id)

	store := NeighborManage.NewMemoryNeighborStore()
	logger := Logger.NewLogger()
	router := Router.NewFanoutRouter()
	smgr := StorageManage.NewStorageManage(
		Storage.NewLocalStorage(storageSlots, storageTTL),
	)

	node := NodeManage.NewDoubleStreamNode(
		nodeHash,
		router,
		logger,
		store,
		factory,
		smgr,
	)

	if err := node.StartGRPCServer(port); err != nil {
		panic(fmt.Sprintf("failed to start gRPC server on %s: %v", port, err))
	}

	return node, nodeHash
}
func BenchmarkTwoNodesMessaging(b *testing.B) {
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()

	node1, node1Hash := newDefaultNode(factory, ":50051", 4, 100*time.Second)
	defer node1.StopGRPC()

	node2, node2Hash := newDefaultNode(factory, ":50052", 4, 100*time.Second)
	defer node2.StopGRPC()

	time.Sleep(500 * time.Millisecond)

	if err := node1.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node2Hash,
		Endpoint: "localhost:50052",
		Online:   true,
	}); err != nil {
		b.Fatalf("Node1 connect failed: %v", err)
	}

	if err := node2.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node1Hash,
		Endpoint: "localhost:50051",
		Online:   true,
	}); err != nil {
		b.Fatalf("Node2 connect failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := &pb.GossipMessage{
			Hash:     fmt.Sprintf("msg-%d", i),
			FromHash: node1Hash,
			PayLoad:  []byte(fmt.Sprintf("test payload %d", i)),
		}

		if err := node1.SendMessage(node2Hash, msg); err != nil {
			b.Errorf("send failed: %v", err)
		}

		select {
		case completedHash := <-node1.MM.CompleteChan:
			if completedHash != msg.Hash {
				b.Fatalf("ACK mismatch: got %s want %s", completedHash, msg.Hash)
			}
		case <-time.After(10 * time.Second):
			b.Fatalf("timeout waiting for ACK")
		}
	}

	b.StopTimer()
}
func BenchmarkBidirectionalMessaging(b *testing.B) {
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()

	node1, node1Hash := newDefaultNode(factory, ":50051", 40, 100*time.Second)
	defer node1.StopGRPC()

	node2, node2Hash := newDefaultNode(factory, ":50052", 40, 100*time.Second)
	defer node2.StopGRPC()

	time.Sleep(500 * time.Millisecond)

	_ = node1.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node2Hash,
		Endpoint: "localhost:50052",
		Online:   true,
	})
	_ = node2.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node1Hash,
		Endpoint: "localhost:50051",
		Online:   true,
	})

	time.Sleep(500 * time.Millisecond)
	b.ResetTimer()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < b.N/2; i++ {
			msg := &pb.GossipMessage{
				Hash:     fmt.Sprintf("node1-msg-%d", i),
				FromHash: node1Hash,
				PayLoad:  []byte("from node1"),
			}
			_ = node1.SendMessage(node2Hash, msg)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < b.N/2; i++ {
			msg := &pb.GossipMessage{
				Hash:     fmt.Sprintf("node2-msg-%d", i),
				FromHash: node2Hash,
				PayLoad:  []byte("from node2"),
			}
			_ = node2.SendMessage(node1Hash, msg)
		}
	}()

	wg.Wait()
}
func TestTwoNodesBasicCommunication(t *testing.T) {
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()

	node1, node1Hash := newDefaultNode(factory, ":50051", 40, 100*time.Second)
	defer node1.StopGRPC()

	node2, node2Hash := newDefaultNode(factory, ":50052", 40, 100*time.Second)
	defer node2.StopGRPC()

	time.Sleep(500 * time.Millisecond)

	_ = node1.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node2Hash,
		Endpoint: "localhost:50052",
		Online:   true,
	})
	_ = node2.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node1Hash,
		Endpoint: "localhost:50051",
		Online:   true,
	})

	time.Sleep(500 * time.Millisecond)

	const numMessages = 15000
	for i := 0; i < numMessages; i++ {
		msg := &pb.GossipMessage{
			Hash:     fmt.Sprintf("test-msg-%d", i),
			FromHash: node1Hash,
			PayLoad:  []byte("hello"),
		}
		_ = node1.SendMessage(node2Hash, msg)
	}

	timeout := time.After(20 * time.Second)
	received := 0

	for received < numMessages {
		select {
		case <-node1.MM.CompleteChan:
			received++
		case <-timeout:
			t.Fatalf("timeout: got %d/%d ACKs", received, numMessages)
		}
	}

	t.Logf("All %d messages ACKed", numMessages)
}

func BenchmarkMultiNodeLargeMessage(b *testing.B) {
	const (
		nodeCount       = 4          // 节点数量（？>=3）
		messagesPerPeer = 30         // 每个节点给每个邻居发 ？ 条
		payloadSize     = 512 * 1024 // ？KB
		basePort        = 51000
		storageSlots    = 200
		storageTTL      = 120 * time.Second
	)

	factory := GossipStreamFactory.NewDoubleStreamFactory(60 * time.Second)
	defer factory.CloseAll()

	// ==========================
	// 1. 创建节点
	// ==========================

	nodes := make([]*NodeManage.DoubleStreamNode, 0, nodeCount)
	nodeHashes := make([]string, 0, nodeCount)
	ports := make([]string, 0, nodeCount)

	for i := 0; i < nodeCount; i++ {
		port := fmt.Sprintf(":%d", basePort+i)
		node, hash := newDefaultNode(factory, port, storageSlots, storageTTL)

		nodes = append(nodes, node)
		nodeHashes = append(nodeHashes, hash)
		ports = append(ports, port)
	}

	defer func() {
		for _, n := range nodes {
			n.StopGRPC()
		}
	}()

	time.Sleep(800 * time.Millisecond)

	// ==========================
	// 2. 全互连
	// ==========================

	for i := 0; i < nodeCount; i++ {
		for j := 0; j < nodeCount; j++ {
			if i == j {
				continue
			}

			err := nodes[i].ConnectToNeighbor(NeighborManage.NeighborInfo{
				NodeHash: nodeHashes[j],
				Endpoint: "localhost" + ports[j],
				Online:   true,
			})
			if err != nil {
				b.Fatalf("connect failed: %v", err)
			}
		}
	}

	time.Sleep(800 * time.Millisecond)

	// ==========================
	// 3. 准备大 payload
	// ==========================

	largePayload := make([]byte, payloadSize)
	for i := range largePayload {
		largePayload[i] = byte(i % 251)
	}

	totalMessages := nodeCount * (nodeCount - 1) * messagesPerPeer
	b.Logf(
		"Multi-node large message benchmark: nodes=%d, payload=%dKB, totalMessages=%d",
		nodeCount,
		payloadSize/1024,
		totalMessages,
	)

	// ==========================
	// 4. 开始压测
	// ==========================

	b.ResetTimer()

	var wgSend sync.WaitGroup
	wgSend.Add(nodeCount)

	for i := 0; i < nodeCount; i++ {
		go func(senderIdx int) {
			defer wgSend.Done()

			sender := nodes[senderIdx]
			senderHash := nodeHashes[senderIdx]

			// ← 对每个邻居也并发发送
			var wgPeer sync.WaitGroup
			for j := 0; j < nodeCount; j++ {
				if senderIdx == j {
					continue
				}

				wgPeer.Add(1)
				go func(peerIdx int) {
					defer wgPeer.Done()
					receiverHash := nodeHashes[peerIdx]

					// 向这个邻居串行发送 30 条
					for k := 0; k < messagesPerPeer; k++ {
						msgHash := fmt.Sprintf("large-%s-to-%s-%d",
							senderHash, receiverHash, k)

						msg := &pb.GossipMessage{
							Hash:     msgHash,
							FromHash: senderHash,
							PayLoad:  largePayload,
						}

						if err := sender.SendMessage(receiverHash, msg); err != nil {
							b.Errorf("send failed: %v", err)
						}
					}
				}(j)
			}
			wgPeer.Wait()
		}(i)
	}

	wgSend.Wait()
	// ==========================
	// 5. 等待所有 ACK
	// ==========================

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ackReceived := int32(0)

	var wgAck sync.WaitGroup
	wgAck.Add(nodeCount)

	for i := 0; i < nodeCount; i++ {
		go func(idx int) {
			defer wgAck.Done()
			node := nodes[idx]

			for {
				select {
				case <-node.MM.CompleteChan:
					if atomic.AddInt32(&ackReceived, 1) >= int32(totalMessages) {
						cancel() // 收到足够的 ACK 后立即取消
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	// 等待完成
	wgAck.Wait()
	cancel() // 确保取消

	final := atomic.LoadInt32(&ackReceived)
	if final < int32(totalMessages) {
		b.Fatalf("ACK incomplete: received %d / %d", final, totalMessages)
	}

	b.Logf("All ACKs received: %d", final)
	b.StopTimer()
}

// 缓存穿透、缓存雪崩……面试人很喜欢问
