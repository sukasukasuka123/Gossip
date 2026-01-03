package test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/sukasukasuka123/Gossip/NodeManage"
)

// 1. 测试节点创建和启动
func TestNodeCreationAndStart(t *testing.T) {
	cfg := NodeManage.DefaultNodeConfig()
	cfg.Port = 7000

	node := NodeManage.NewChunkNode(cfg)
	if node == nil {
		t.Fatal("Failed to create node")
	}

	err := node.Start()
	if err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	// 确保节点正常运行
	time.Sleep(100 * time.Millisecond)

	node.Stop()
}

// 2. 测试多个节点的创建和停止
func TestMultipleNodesCreation(t *testing.T) {
	nodes := make([]*NodeManage.ChunkNode, 3)
	basePort := 7100

	// 创建并启动节点
	for i := 0; i < 3; i++ {
		cfg := NodeManage.DefaultNodeConfig()
		cfg.Port = basePort + i
		nodes[i] = NodeManage.NewChunkNode(cfg)

		if nodes[i] == nil {
			t.Fatalf("Failed to create node %d", i)
		}

		err := nodes[i].Start()
		if err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// 按照相反顺序关闭节点（重要：保持这个顺序）
	for i := 2; i >= 0; i-- {
		nodes[i].Stop()
	}
}

// 3. 测试节点间连接
func TestNodeConnection(t *testing.T) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 8001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 8002

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)

	err := nodeA.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeA: %v", err)
	}

	err = nodeB.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeB: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 建立连接
	err = nodeA.ConnectToNeighbor("127.0.0.1", 8002)
	if err != nil {
		t.Fatalf("Failed to connect nodes: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 验证连接状态
	stats := nodeA.GetStats()
	if stats == nil {
		t.Fatal("Failed to get stats from nodeA")
	}

	// 按照相反顺序关闭（重要）
	nodeA.Stop()
	nodeB.Stop()

}

// 4. 测试消息广播功能
func TestMessageBroadcast(t *testing.T) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 9001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 9002

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)

	err := nodeA.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeA: %v", err)
	}

	err = nodeB.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeB: %v", err)
	}

	err = nodeA.ConnectToNeighbor("127.0.0.1", 9002)
	if err != nil {
		t.Fatalf("Failed to connect nodes: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 发送消息
	testPayload := []byte("TEST_MESSAGE")
	nodeA.BroadcastMessage(testPayload)

	// 等待消息传播
	time.Sleep(300 * time.Millisecond)

	// 验证接收
	statsB := nodeB.GetStats()
	if statsB != nil {
		if received, ok := statsB["messages_received"].(int64); ok {
			if received == 0 {
				t.Error("NodeB did not receive any messages")
			} else {
				t.Logf("NodeB received %d messages", received)
			}
		}
	}

	// 按照相反顺序关闭（重要）
	nodeA.Stop()
	nodeB.Stop()

}

// 5. 测试小消息广播
func TestSmallMessageBroadcast(t *testing.T) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 10001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 10002
	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 10003

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	err := nodeA.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeA: %v", err)
	}

	err = nodeB.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeB: %v", err)
	}

	err = nodeC.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeC: %v", err)
	}

	err = nodeA.ConnectToNeighbor("127.0.0.1", 10002)
	if err != nil {
		t.Fatalf("Failed to connect to nodeB: %v", err)
	}

	err = nodeA.ConnectToNeighbor("127.0.0.1", 10003)
	if err != nil {
		t.Fatalf("Failed to connect to nodeC: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 发送小消息
	payload := []byte("SMALL_MESSAGE")
	nodeA.BroadcastMessage(payload)

	time.Sleep(300 * time.Millisecond)

	// 验证接收
	statsB := nodeB.GetStats()
	statsC := nodeC.GetStats()

	t.Logf("NodeB stats: %v", statsB)
	t.Logf("NodeC stats: %v", statsC)

	// 按照相反顺序关闭（重要）
	nodeA.Stop()
	nodeC.Stop()
	nodeB.Stop()

}

// 6. 测试大消息广播
func TestLargeMessageBroadcast(t *testing.T) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 11001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 11002
	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 11003

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	err := nodeA.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeA: %v", err)
	}

	err = nodeB.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeB: %v", err)
	}

	err = nodeC.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeC: %v", err)
	}

	err = nodeA.ConnectToNeighbor("127.0.0.1", 11002)
	if err != nil {
		t.Fatalf("Failed to connect to nodeB: %v", err)
	}

	err = nodeA.ConnectToNeighbor("127.0.0.1", 11003)
	if err != nil {
		t.Fatalf("Failed to connect to nodeC: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 发送大消息 (约10KB)
	payload := bytes.Repeat([]byte("LARGE_DATA"), 1024)
	nodeA.BroadcastMessage(payload)

	time.Sleep(500 * time.Millisecond)

	// 验证接收
	statsB := nodeB.GetStats()
	statsC := nodeC.GetStats()

	if statsB != nil {
		if received, ok := statsB["messages_received"].(int64); ok {
			t.Logf("NodeB received %d messages", received)
		}
	}

	if statsC != nil {
		if received, ok := statsC["messages_received"].(int64); ok {
			t.Logf("NodeC received %d messages", received)
		}
	}

	// 按照相反顺序关闭（重要）
	nodeA.Stop()
	nodeC.Stop()
	nodeB.Stop()
}
func TestMultiNodeStarTopology(t *testing.T) {
	nodeCount := 5
	nodes := make([]*NodeManage.ChunkNode, nodeCount)
	basePort := 12001

	// 创建并启动所有节点
	for i := 0; i < nodeCount; i++ {
		cfg := NodeManage.DefaultNodeConfig()
		cfg.Port = basePort + i
		nodes[i] = NodeManage.NewChunkNode(cfg)

		if err := nodes[i].Start(); err != nil {
			t.Fatalf("Failed to start node %d: %v", i, err)
		}
	}

	defer func() {
		for i := 0; i < nodeCount; i++ {
			nodes[i].Stop()
		}
	}()

	// 星型拓扑：0 -> others
	for i := 1; i < nodeCount; i++ {
		if err := nodes[0].ConnectToNeighbor("127.0.0.1", basePort+i); err != nil {
			t.Fatalf("Failed to connect node 0 to node %d: %v", i, err)
		}
	}

	// 给连接一点时间稳定
	time.Sleep(200 * time.Millisecond)

	// 发送消息
	payload := bytes.Repeat([]byte("MULTI_NODE"), 256)
	nodes[0].BroadcastMessage(payload)

	// 等待所有节点“最终”收到消息
	waitUntil(t, 2*time.Second, func() bool {
		for i := 1; i < nodeCount; i++ {
			stats := nodes[i].GetStats()
			if stats == nil {
				return false
			}
			received, ok := stats["messages_received"].(int64)
			if !ok || received < 1 {
				return false
			}
		}
		return true
	})

	// 通过后再打日志（方便调试）
	for i := 1; i < nodeCount; i++ {
		stats := nodes[i].GetStats()
		t.Logf("Node %d received %d messages",
			i, stats["messages_received"].(int64))
	}
}

// 8. 测试重复广播
func TestMultipleBroadcasts(t *testing.T) {
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 13001
	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 13002

	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)

	err := nodeA.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeA: %v", err)
	}

	err = nodeB.Start()
	if err != nil {
		t.Fatalf("Failed to start nodeB: %v", err)
	}

	err = nodeA.ConnectToNeighbor("127.0.0.1", 13002)
	if err != nil {
		t.Fatalf("Failed to connect nodes: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// 多次广播
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		payload := []byte("MESSAGE_" + string(rune(i)))
		nodeA.BroadcastMessage(payload)
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	// 验证接收
	statsB := nodeB.GetStats()
	if statsB != nil {
		if received, ok := statsB["messages_received"].(int64); ok {
			t.Logf("NodeB received %d messages (expected at least %d)", received, messageCount)
			if received < int64(messageCount) {
				t.Errorf("NodeB received fewer messages than expected: got %d, want at least %d",
					received, messageCount)
			}
		}
	}

	// 按照相反顺序关闭（重要）
	nodeA.Stop()
	nodeB.Stop()

}

// 9. 测试节点统计信息
func TestNodeStats(t *testing.T) {
	cfg := NodeManage.DefaultNodeConfig()
	cfg.Port = 14001

	node := NodeManage.NewChunkNode(cfg)
	err := node.Start()
	if err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 获取统计信息
	stats := node.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}

	t.Logf("Node stats: %v", stats)

	node.Stop()
}

// 10. 测试连接失败情况
func TestConnectionFailure(t *testing.T) {
	cfg := NodeManage.DefaultNodeConfig()
	cfg.Port = 15001

	node := NodeManage.NewChunkNode(cfg)
	err := node.Start()
	if err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// 尝试连接到不存在的节点
	err = node.ConnectToNeighbor("127.0.0.1", 19999)
	if err == nil {
		t.Log("Connection to non-existent node handled (may or may not return error)")
	} else {
		t.Logf("Connection failed as expected: %v", err)
	}

	node.Stop()
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
func TestChunkGossipStressStability(t *testing.T) {
	// ===============================
	// 参数配置（偏向压力，但仍是 unit test）
	// ===============================
	nodeCount := 14
	basePort := 14000
	messageRepeat := 1024 * 128
	broadcastRounds := 50

	// ===============================
	// 创建并启动节点
	// ===============================
	nodes := make([]*NodeManage.ChunkNode, nodeCount)

	for i := 0; i < nodeCount; i++ {
		cfg := NodeManage.DefaultNodeConfig()
		cfg.Port = basePort + i
		nodes[i] = NodeManage.NewChunkNode(cfg)
		if err := nodes[i].Start(); err != nil {
			t.Fatalf("failed to start node %d: %v", i, err)
		}
	}

	for i := nodeCount - 1; i >= 0; i-- {
		defer nodes[i].Stop()
	}

	// ===============================
	// 构建半 Mesh 拓扑
	// ===============================
	for i := 1; i < nodeCount; i++ {
		_ = nodes[0].ConnectToNeighbor("127.0.0.1", basePort+i)
	}

	for i := 1; i < nodeCount; i++ {
		for j := i + 1; j < nodeCount; j++ {
			if (i+j)%2 == 0 {
				_ = nodes[i].ConnectToNeighbor("127.0.0.1", basePort+j)
			}
		}
	}

	time.Sleep(500 * time.Millisecond)

	// ===============================
	// 构造压力消息
	// ===============================
	payload := bytes.Repeat(
		[]byte("UNIT_TEST_STRESS_PAYLOAD_"),
		messageRepeat,
	)

	// ===============================
	// 并发广播（但数量受控）
	// ===============================
	var wg sync.WaitGroup
	concurrency := 4
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < broadcastRounds; j++ {
				nodes[0].BroadcastMessage(payload)
			}
		}()
	}

	wg.Wait()

	// 给 Gossip 一点时间完成传播
	time.Sleep(800 * time.Millisecond)

	// ===============================
	// 验证：系统是否“合理收敛”
	// ===============================
	for i := 1; i < nodeCount; i++ {
		stats := nodes[i].GetStats()

		received, ok := stats["messages_received"].(int64)
		if !ok {
			t.Fatalf("node %d has no messages_received stat", i)
		}

		// 不是精确断言，而是下限断言
		if received == 0 {
			t.Fatalf("node %d received 0 messages under stress", i)
		}

		t.Logf(
			"node %d received %d messages under stress",
			i,
			received,
		)
	}
}
