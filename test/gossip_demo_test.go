package test

import (
	"fmt"
	"log"
	"sync"
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

// BenchmarkTwoNodesMessaging 测试两个节点之间的消息传递性能
func BenchmarkTwoNodesMessaging(b *testing.B) {
	// 创建流工厂
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()
	// 节点1配置
	node1Hash := "node1"
	node1Port := ":50051"
	store1 := NeighborManage.NewMemoryNeighborStore()
	logger1 := Logger.NewLogger()
	router1 := Router.NewFanoutRouter()
	smgr1 := StorageManage.NewStorageManage(Storage.NewLocalStorage(4, 100*time.Second))
	node1 := NodeManage.NewDoubleStreamNode(node1Hash, router1, logger1, store1, factory, smgr1)
	defer node1.StopGRPC()
	// 节点2配置
	node2Hash := "node2"
	node2Port := ":50052"
	store2 := NeighborManage.NewMemoryNeighborStore()
	logger2 := Logger.NewLogger()
	router2 := Router.NewFanoutRouter()
	smgr2 := StorageManage.NewStorageManage(Storage.NewLocalStorage(4, 100*time.Second))
	node2 := NodeManage.NewDoubleStreamNode(node2Hash, router2, logger2, store2, factory, smgr2)
	defer node2.StopGRPC()
	// 启动gRPC服务器
	if err := node1.StartGRPCServer(node1Port); err != nil {
		b.Fatalf("Failed to start node1 gRPC server: %v", err)
	}
	if err := node2.StartGRPCServer(node2Port); err != nil {
		b.Fatalf("Failed to start node2 gRPC server: %v", err)
	}
	// 等待服务器启动
	time.Sleep(500 * time.Millisecond)
	// 节点1连接到节点2
	err := node1.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node2Hash,
		Endpoint: "localhost" + node2Port,
		Online:   true,
	})
	if err != nil {
		b.Fatalf("Node1 failed to connect to Node2: %v", err)
	}
	// 节点2连接到节点1
	err = node2.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node1Hash,
		Endpoint: "localhost" + node1Port,
		Online:   true,
	})
	if err != nil {
		b.Fatalf("Node2 failed to connect to Node1: %v", err)
	}
	// 等待连接建立
	time.Sleep(500 * time.Millisecond)
	// 等待连接建立
	time.Sleep(500 * time.Millisecond)
	// 重置计时器，排除初始化时间
	b.ResetTimer() // <-- 计时开始
	// 运行基准测试
	for i := 0; i < b.N; i++ {
		msg := &pb.GossipMessage{
			Hash:     fmt.Sprintf("msg-%d", i),
			FromHash: node1Hash,
			PayLoad:  []byte(fmt.Sprintf("test payload %d", i)),
		}
		err := node1.SendMessage(node2Hash, msg)
		if err != nil {
			b.Errorf("Failed to send message: %v", err)
		}
		select {
		case completedHash := <-node1.MM.CompleteChan:
			if completedHash != msg.Hash {
				b.Fatalf("ReceivedACK hash: got %s, expected %s", completedHash, msg.Hash)
			}
			// 成功解除阻塞，进入下一个循环
			// 注意：这里移除了 b.StopTimer()
		case <-time.After(10 * time.Second): // 设置一个合理的等待上限
			b.Fatalf("Message %s failed to receive ACK within timeout", msg.Hash)
		}
	}
	// [关键修改] 在循环结束后，清理操作开始前，停止计时器
	b.StopTimer()
	// 接下来函数返回，defer 语句 (node1.StopGRPC() 等) 开始执行
}

// // BenchmarkParallelMessaging 测试并发消息传递性能
//
//	func BenchmarkParallelMessaging(b *testing.B) {
//		factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
//		defer factory.CloseAll()
//		node1Hash := "node1"
//		node1Port := ":50053"
//		store1 := NeighborManage.NewMemoryNeighborStore()
//		logger1 := Logger.NewLogger()
//		router1 := Router.NewFanoutRouter()
//		node1 := NodeManage.NewDoubleStreamNode(node1Hash, router1, logger1, store1, factory)
//		defer node1.StopGRPC()
//		node2Hash := "node2"
//		node2Port := ":50054"
//		store2 := NeighborManage.NewMemoryNeighborStore()
//		logger2 := Logger.NewLogger()
//		router2 := Router.NewFanoutRouter()
//		node2 := NodeManage.NewDoubleStreamNode(node2Hash, router2, logger2, store2, factory)
//		defer node2.StopGRPC()
//		if err := node1.StartGRPCServer(node1Port); err != nil {
//			b.Fatalf("Failed to start node1 gRPC server: %v", err)
//		}
//		if err := node2.StartGRPCServer(node2Port); err != nil {
//			b.Fatalf("Failed to start node2 gRPC server: %v", err)
//		}
//		time.Sleep(500 * time.Millisecond)
//		err := node1.ConnectToNeighbor(NeighborManage.NeighborInfo{
//			NodeHash: node2Hash,
//			Endpoint: "localhost" + node2Port,
//			Online:   true,
//		})
//		if err != nil {
//			b.Fatalf("Node1 failed to connect to Node2: %v", err)
//		}
//		err = node2.ConnectToNeighbor(NeighborManage.NeighborInfo{
//			NodeHash: node1Hash,
//			Endpoint: "localhost" + node1Port,
//			Online:   true,
//		})
//		if err != nil {
//			b.Fatalf("Node2 failed to connect to Node1: %v", err)
//		}
//		time.Sleep(500 * time.Millisecond)
//		b.ResetTimer()
//		// 并行发送消息
//		b.RunParallel(func(pb *testing.PB) {
//			i := 0
//			for pb.Next() {
//				msg := &pb.GossipMessage{
//					Hash:     fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), i),
//					FromHash: node1Hash,
//					PayLoad:  []byte(fmt.Sprintf("test payload %d", i)),
//				}
//				_ = node1.SendMessage(node2Hash, msg)
//				i++
//			}
//		})
//	}
//
// BenchmarkBidirectionalMessaging 测试双向消息传递
func BenchmarkBidirectionalMessaging(b *testing.B) {
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()
	// 节点1配置
	node1Hash := "node1"
	node1Port := ":50051"
	store1 := NeighborManage.NewMemoryNeighborStore()
	logger1 := Logger.NewLogger()
	router1 := Router.NewFanoutRouter()
	smgr1 := StorageManage.NewStorageManage(Storage.NewLocalStorage(40, 100*time.Second))
	node1 := NodeManage.NewDoubleStreamNode(node1Hash, router1, logger1, store1, factory, smgr1)
	defer node1.StopGRPC()
	// 节点2配置
	node2Hash := "node2"
	node2Port := ":50052"
	store2 := NeighborManage.NewMemoryNeighborStore()
	logger2 := Logger.NewLogger()
	router2 := Router.NewFanoutRouter()
	smgr2 := StorageManage.NewStorageManage(Storage.NewLocalStorage(40, 100*time.Second))
	node2 := NodeManage.NewDoubleStreamNode(node2Hash, router2, logger2, store2, factory, smgr2)
	defer node2.StopGRPC()
	if err := node1.StartGRPCServer(node1Port); err != nil {
		b.Fatalf("Failed to start node1 gRPC server: %v", err)
	}
	if err := node2.StartGRPCServer(node2Port); err != nil {
		b.Fatalf("Failed to start node2 gRPC server: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	err := node1.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node2Hash,
		Endpoint: "localhost" + node2Port,
		Online:   true,
	})
	if err != nil {
		b.Fatalf("Node1 failed to connect to Node2: %v", err)
	}
	err = node2.ConnectToNeighbor(NeighborManage.NeighborInfo{
		NodeHash: node1Hash,
		Endpoint: "localhost" + node1Port,
		Online:   true,
	})
	if err != nil {
		b.Fatalf("Node2 failed to connect to Node1: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(2)
	// 节点1发送消息
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/2; i++ {
			msg := &pb.GossipMessage{
				Hash:     fmt.Sprintf("node1-msg-%d", i),
				FromHash: node1Hash,
				PayLoad:  []byte(fmt.Sprintf("from node1: %d", i)),
			}
			_ = node1.SendMessage(node2Hash, msg)
		}
	}()
	// 节点2发送消息
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/2; i++ {
			msg := &pb.GossipMessage{
				Hash:     fmt.Sprintf("node2-msg-%d", i),
				FromHash: node2Hash,
				PayLoad:  []byte(fmt.Sprintf("from node2: %d", i)),
			}
			_ = node2.SendMessage(node1Hash, msg)
		}
	}()
	wg.Wait()
}

// TestTwoNodesBasicCommunication 基础功能测试
func TestTwoNodesBasicCommunication(t *testing.T) {
	// 设置 Stream Factory，确保连接可以被管理和关闭
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()

	// --- 节点1配置 ---
	node1Hash := "node1"
	node1Port := ":50051"
	store1 := NeighborManage.NewMemoryNeighborStore()
	logger1 := Logger.NewLogger()
	router1 := Router.NewFanoutRouter()
	smgr1 := StorageManage.NewStorageManage(Storage.NewLocalStorage(40, 100*time.Second))
	// 确保 NodeManage 结构体中的 MM 字段类型已修正为指针 (*MessageManager)
	node1 := NodeManage.NewDoubleStreamNode(node1Hash, router1, logger1, store1, factory, smgr1)
	defer node1.StopGRPC()

	// --- 节点2配置 ---
	node2Hash := "node2"
	node2Port := ":50052"
	store2 := NeighborManage.NewMemoryNeighborStore()
	logger2 := Logger.NewLogger()
	router2 := Router.NewFanoutRouter()
	smgr2 := StorageManage.NewStorageManage(Storage.NewLocalStorage(40, 100*time.Second))
	node2 := NodeManage.NewDoubleStreamNode(node2Hash, router2, logger2, store2, factory, smgr2)
	defer node2.StopGRPC()

	// 启动服务器
	if err := node1.StartGRPCServer(node1Port); err != nil {
		t.Fatalf("Failed to start node1 gRPC server: %v", err)
	}
	if err := node2.StartGRPCServer(node2Port); err != nil {
		t.Fatalf("Failed to start node2 gRPC server: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 建立连接
	t.Log("Establishing connection between node1 and node2...")
	err := node1.ConnectToNeighbor(NeighborManage.NeighborInfo{NodeHash: node2Hash, Endpoint: "localhost" + node2Port, Online: true})
	if err != nil {
		t.Fatalf("Node1 failed to connect to Node2: %v", err)
	}
	err = node2.ConnectToNeighbor(NeighborManage.NeighborInfo{NodeHash: node1Hash, Endpoint: "localhost" + node1Port, Online: true})
	if err != nil {
		t.Fatalf("Node2 failed to connect to Node1: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	t.Log("Connection established.")

	// --- 1. 定义测试消息 ---
	const numMessages = 15000
	messages := make([]*pb.GossipMessage, numMessages)

	// --- 2. 节点1 发送大量消息给节点2 ---
	t.Logf("Node1 sending %d messages to Node2...", numMessages)
	for i := 0; i < numMessages; i++ {
		hash := fmt.Sprintf("test-msg-%d", i)
		msg := &pb.GossipMessage{
			Hash:     hash,
			FromHash: node1Hash,
			PayLoad:  []byte(fmt.Sprintf("Hello from Node1, message %d", i)),
		}
		messages[i] = msg

		if err := node1.SendMessage(node2Hash, msg); err != nil {
			t.Errorf("Failed to send message %s: %v", hash, err)
		}
	}
	t.Logf("%d messages sent from Node1 to Node2.", numMessages)

	// --- 3. 等待所有 ACK 接收 (Node1 收到 ACK) ---
	t.Logf("Waiting for %d ACKs on node1.MM.CompleteChan...", numMessages)
	var sum int
	sum = 0
	timeout := time.After(20 * time.Second) // 增加超时时间以应对大量消息

	for sum < numMessages {
		select {
		case completedHash := <-node1.MM.CompleteChan:
			sum++
			log.Printf("Node1 received ACK for %s (%d/%d)\n", completedHash, sum, numMessages)
		case <-timeout:
			t.Fatalf("Timeout: Node1 failed to receive all %d ACKs within 15 seconds. Received %d.", numMessages, sum)
		}
	}

	t.Logf("Communication test successful. All %d messages sent, received by Node2, and ACKs confirmed by Node1.", numMessages)
}

// BenchmarkStreamFactory 测试流工厂的性能
func BenchmarkStreamFactory(b *testing.B) {
	factory := GossipStreamFactory.NewDoubleStreamFactory(30 * time.Second)
	defer factory.CloseAll()
	// 启动一个临时服务器用于连接
	logger := Logger.NewLogger()
	router := Router.NewFanoutRouter()
	smgr := StorageManage.NewStorageManage(Storage.NewLocalStorage(40, 100*time.Second))
	node := NodeManage.NewDoubleStreamNode("test-node", router, logger,
		NeighborManage.NewMemoryNeighborStore(), factory, smgr)
	defer node.StopGRPC()
	if err := node.StartGRPCServer(":50059"); err != nil {
		b.Fatalf("Failed to start test server: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	b.ResetTimer()
	// 测试流的获取性能
	for i := 0; i < b.N; i++ {
		_, err := factory.GetDoubleStream(fmt.Sprintf("node-%d", i%10), "localhost:50059")
		if err != nil {
			b.Errorf("Failed to get stream: %v", err)
		}
	}
}
