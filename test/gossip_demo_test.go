package test

import (
	"bytes"
	"testing"
	"time"

	"github.com/sukasukasuka123/Gossip/NodeManage" // 替换为你的实际包路径
)

// 1. 测试节点创建和启动的性能
func BenchmarkNodeCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cfg := NodeManage.DefaultNodeConfig()
		cfg.Port = 7000 + i%1000 // 避免端口冲突

		node := NodeManage.NewChunkNode(cfg)
		_ = node.Start()
		node.Stop()
	}
}

// 2. 测试建立连接的性能
func BenchmarkNodeConnection(b *testing.B) {
	// 准备两个固定节点
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 8001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 8002

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)

	_ = nodeA.Start()
	_ = nodeB.Start()
	defer nodeB.Stop()
	defer nodeA.Stop()

	time.Sleep(100 * time.Millisecond) // 等待节点就绪

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 测试连接建立（注意：多次连接到同一节点可能需要你的代码支持重连逻辑）
		_ = nodeA.ConnectToNeighbor("127.0.0.1", 8002)
	}
}

// 3. 测试单次消息发送的性能（点对点）
func BenchmarkSingleMessageSend(b *testing.B) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 9001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 9002

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)

	_ = nodeA.Start()
	_ = nodeB.Start()
	defer nodeB.Stop()
	defer nodeA.Stop()

	_ = nodeA.ConnectToNeighbor("127.0.0.1", 9002)
	time.Sleep(200 * time.Millisecond)

	payload := bytes.Repeat([]byte("TEST_DATA"), 128) // 1KB左右的数据

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodeA.BroadcastMessage(payload)
	}
}

// 4. 测试广播性能（小消息）
func BenchmarkBroadcastSmallMessage(b *testing.B) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 10001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 10002
	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 10003

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	_ = nodeA.Start()
	_ = nodeB.Start()
	_ = nodeC.Start()
	defer nodeB.Stop()
	defer nodeC.Stop()
	defer nodeA.Stop()

	_ = nodeA.ConnectToNeighbor("127.0.0.1", 10002)
	_ = nodeA.ConnectToNeighbor("127.0.0.1", 10003)
	time.Sleep(200 * time.Millisecond)

	payload := []byte("SMALL_MESSAGE") // 小消息

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodeA.BroadcastMessage(payload)
	}
}

// 5. 测试广播性能（大消息）
func BenchmarkBroadcastLargeMessage(b *testing.B) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 11001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 11002
	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 11003

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	_ = nodeA.Start()
	_ = nodeB.Start()
	_ = nodeC.Start()
	defer nodeB.Stop()
	defer nodeC.Stop()
	defer nodeA.Stop()
	_ = nodeA.ConnectToNeighbor("127.0.0.1", 11002)
	_ = nodeA.ConnectToNeighbor("127.0.0.1", 11003)
	time.Sleep(200 * time.Millisecond)

	payload := bytes.Repeat([]byte("LARGE_DATA"), 1024) // 10KB左右的数据

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodeA.BroadcastMessage(payload)
	}
}

// 6. 测试并发广播性能（限制2核心）
func BenchmarkChunkGossip_Realistic(b *testing.B) {
	// 设置只使用2个CPU核心
	b.SetParallelism(2)

	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 6001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 6002
	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 6003

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	_ = nodeA.Start()
	_ = nodeB.Start()
	_ = nodeC.Start()

	defer nodeB.Stop()
	defer nodeC.Stop()
	defer nodeA.Stop()
	_ = nodeA.ConnectToNeighbor("127.0.0.1", 6002)
	_ = nodeA.ConnectToNeighbor("127.0.0.1", 6003)

	time.Sleep(200 * time.Millisecond)

	payload := bytes.Repeat([]byte("REALISTIC_DATA"), 512) // 7KB左右

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			nodeA.BroadcastMessage(payload)
		}
	})

	b.StopTimer()

	statsB := nodeB.GetStats()["messages_received"].(int64)
	statsC := nodeC.GetStats()["messages_received"].(int64)
	b.Logf("Total Broadcasts: %d, NodeB Received: %d, NodeC Received: %d", b.N, statsB, statsC)
}

// 7. 测试多节点场景下的性能（扩展测试）
func BenchmarkMultiNodeBroadcast(b *testing.B) {
	nodeCount := 5
	nodes := make([]*NodeManage.ChunkNode, nodeCount)
	basePort := 12001

	// 创建并启动所有节点
	for i := 0; i < nodeCount; i++ {
		cfg := NodeManage.DefaultNodeConfig()
		cfg.Port = basePort + i
		nodes[i] = NodeManage.NewChunkNode(cfg)
		_ = nodes[i].Start()
	}
	for i := nodeCount - 1; i >= 0; i-- {
		defer nodes[i].Stop()
	}
	// 将所有节点连接到第一个节点（星型拓扑）
	for i := 1; i < nodeCount; i++ {
		_ = nodes[0].ConnectToNeighbor("127.0.0.1", basePort+i)
	}

	time.Sleep(300 * time.Millisecond)

	payload := bytes.Repeat([]byte("MULTI_NODE"), 256)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodes[0].BroadcastMessage(payload)
	}
}
func BenchmarkChunkGossip_Stress(b *testing.B) {
	// ===============================
	// 基准测试-压测参数（可以按需调）
	// ===============================
	nodeCount := 14             // 节点数量
	parallelism := 4            // 并发广播 goroutine 倍数
	messageRepeat := 1024 * 128 // 单条消息大小倍率
	basePort := 13000

	b.SetParallelism(parallelism)

	// ===============================
	// 创建并启动节点
	// ===============================
	nodes := make([]*NodeManage.ChunkNode, nodeCount)

	for i := 0; i < nodeCount; i++ {
		cfg := NodeManage.DefaultNodeConfig()
		cfg.Port = basePort + i
		nodes[i] = NodeManage.NewChunkNode(cfg)
		_ = nodes[i].Start()
	}

	for i := nodeCount - 1; i >= 0; i-- {
		defer nodes[i].Stop()
	}

	// ===============================
	// 构建拓扑：半 Mesh（比星型更残酷）
	// Node0 连接所有
	// 其他节点互连一部分
	// ===============================
	for i := 1; i < nodeCount; i++ {
		_ = nodes[0].ConnectToNeighbor("127.0.0.1", basePort+i)
	}

	for i := 1; i < nodeCount; i++ {
		for j := i + 1; j < nodeCount; j++ {
			if (i+j)%2 == 0 { // 控制连接密度
				_ = nodes[i].ConnectToNeighbor("127.0.0.1", basePort+j)
			}
		}
	}

	// 等待网络稳定
	time.Sleep(500 * time.Millisecond)

	// ===============================
	// 构造“偏真实但恶意”的消息
	// ===============================
	payload := bytes.Repeat(
		[]byte("STRESS_GOSSIP_PAYLOAD_"),
		messageRepeat, //
	)

	// ===============================
	// 开始压测
	// ===============================
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 固定由 node0 发起，最大化 fan-out 压力
			nodes[0].BroadcastMessage(payload)
		}
	})

	b.StopTimer()

	// ===============================
	// 统计结果（不计入性能）
	// ===============================
	totalReceived := int64(0)
	for i := 1; i < nodeCount; i++ {
		stats := nodes[i].GetStats()
		if v, ok := stats["messages_received"].(int64); ok {
			totalReceived += v
		}
	}

	b.Logf(
		"[Stress Result] Nodes=%d Parallel=%d Payload≈%dB TotalReceived=%d",
		nodeCount,
		parallelism,
		len(payload),
		totalReceived,
	)
}
