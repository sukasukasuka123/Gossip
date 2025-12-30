package test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/sukasukasuka123/Gossip/NodeManage"
)

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestChunkGossipIntegration(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())

	defer cancel()

	// ---------- 创建节点配置 ----------
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 6001
	cfgA.FanoutCount = 2

	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 6002

	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 6003

	// ---------- 创建节点 ----------
	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	// ---------- 启动节点 ----------
	if err := nodeA.Start(); err != nil {
		t.Fatal(err)
	}
	if err := nodeB.Start(); err != nil {
		t.Fatal(err)
	}
	if err := nodeC.Start(); err != nil {
		t.Fatal(err)
	}

	defer nodeB.Stop()
	defer nodeC.Stop()
	defer nodeA.Stop()
	// ---------- 建立邻居关系 ----------
	if err := nodeA.ConnectToNeighbor("127.0.0.1", cfgB.Port); err != nil {
		t.Fatal(err)
	}
	if err := nodeA.ConnectToNeighbor("127.0.0.1", cfgC.Port); err != nil {
		t.Fatal(err)
	}

	// 等待连接稳定
	time.Sleep(500 * time.Millisecond)

	// ---------- 准备测试消息 ----------
	payload := bytes.Repeat([]byte("HELLO_GOSSIP"), 1024) // > chunkSize
	payloadHash := nodeA.BroadcastMessage(payload)

	t.Logf("broadcast payload hash: %s", payloadHash)

	// ---------- 等待 B / C 收到 ----------
	waitUntil(t, 5*time.Second, func() bool {
		statsB := nodeB.GetStats()
		statsC := nodeC.GetStats()

		return statsB["messages_received"].(int64) >= 1 &&
			statsC["messages_received"].(int64) >= 1
	})

	// ---------- 验证统计 ----------
	statsA := nodeA.GetStats()
	statsB := nodeB.GetStats()
	statsC := nodeC.GetStats()

	t.Logf("A stats: %+v", statsA)
	t.Logf("B stats: %+v", statsB)
	t.Logf("C stats: %+v", statsC)

	if statsA["messages_sent"].(int64) != 1 {
		t.Fatalf("expected A sent 1 message, got %v", statsA["messages_sent"])
	}

	if statsB["messages_received"].(int64) != 1 {
		t.Fatalf("expected B received 1 message, got %v", statsB["messages_received"])
	}

	if statsC["messages_received"].(int64) != 1 {
		t.Fatalf("expected C received 1 message, got %v", statsC["messages_received"])
	}
}
func BenchmarkChunkGossip(b *testing.B) {
	// 记录内存分配
	b.ReportAllocs()

	// ---------- 创建节点配置 ----------
	cfgA := NodeManage.DefaultNodeConfig()
	cfgA.Port = 6001
	cfgA.FanoutCount = 2

	cfgB := NodeManage.DefaultNodeConfig()
	cfgB.Port = 6002

	cfgC := NodeManage.DefaultNodeConfig()
	cfgC.Port = 6003

	// ---------- 创建节点 ----------
	nodeA := NodeManage.NewChunkNode(cfgA)
	nodeB := NodeManage.NewChunkNode(cfgB)
	nodeC := NodeManage.NewChunkNode(cfgC)

	// ---------- 启动节点 ----------
	if err := nodeA.Start(); err != nil {
		b.Fatal(err)
	}
	if err := nodeB.Start(); err != nil {
		b.Fatal(err)
	}
	if err := nodeC.Start(); err != nil {
		b.Fatal(err)
	}

	defer nodeB.Stop()
	defer nodeC.Stop()
	defer nodeA.Stop()

	// ---------- 建立邻居关系 ----------
	if err := nodeA.ConnectToNeighbor("127.0.0.1", cfgB.Port); err != nil {
		b.Fatal(err)
	}
	if err := nodeA.ConnectToNeighbor("127.0.0.1", cfgC.Port); err != nil {
		b.Fatal(err)
	}

	// 等待连接稳定
	time.Sleep(500 * time.Millisecond)

	payload := bytes.Repeat([]byte("HELLO_GOSSIP"), 1024) // > chunkSize

	// ---------- 基准循环 ----------
	for i := 0; i < b.N; i++ {
		hash := nodeA.BroadcastMessage(payload)

		// 等待 B / C 收到
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			statsB := nodeB.GetStats()
			statsC := nodeC.GetStats()

			if statsB["messages_received"].(int64) >= int64(i+1) &&
				statsC["messages_received"].(int64) >= int64(i+1) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		// 可选：打印每轮 hash
		b.Logf("iteration %d broadcast payload hash: %s", i, hash)
	}

	// ---------- 输出最终统计 ----------
	statsA := nodeA.GetStats()
	statsB := nodeB.GetStats()
	statsC := nodeC.GetStats()

	b.Logf("A stats: %+v", statsA)
	b.Logf("B stats: %+v", statsB)
	b.Logf("C stats: %+v", statsC)
}
