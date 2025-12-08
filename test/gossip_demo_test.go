// 文件路径: Gossip/test/gossip_demo_test.go
package test

import (
	"testing"
	"time"

	"github.com/sukasukasuka123/Gossip/NodeManage"
	"github.com/sukasukasuka123/Gossip/NodeManage/Logger"
	"github.com/sukasukasuka123/Gossip/NodeManage/Router"
	"github.com/sukasukasuka123/Gossip/NodeManage/Storage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc/proto"
)

// TestGossipNodeDemo 是一个演示如何使用 GossipNode 的最小单元测试示例
func TestGossipNodeDemo(t *testing.T) {
	// ----------步骤1：准备存储、路由和日志----------
	store := Storage.NewLocalStorage(50, 2*time.Minute)
	router := Router.NewFanoutRouter()
	log := Logger.NewLogger()

	// ----------步骤2：创建节点----------
	node1 := NodeManage.NewGossipNode("node1", store, router, log, 5*time.Second)
	node2 := NodeManage.NewGossipNode("node2", store, router, log, 5*time.Second)

	// ----------步骤3：启动 gRPC server----------
	node1.StartGRPCServer(":7001")
	node2.StartGRPCServer(":7002")

	// 等待 gRPC server 启动
	time.Sleep(300 * time.Millisecond)

	// ----------步骤4：节点1注册节点2----------
	node1.Neighbors["node2"] = "localhost:7002"
	// ----------步骤5：节点2注册节点1----------
	node2.Neighbors["node1"] = "localhost:7001"

	// ----------步骤6：节点1发送 JSON 格式消息到节点2----------
	jsonMsg := &pb.GossipMessage{
		Hash:     "msg-json-01",
		FromHash: node1.NodeHash,
		PayLoad:  []byte(`{"type":"greeting","content":"Hello from node1"}`),
	}
	err := node1.SendMessageToNode("node2", jsonMsg)
	if err != nil {
		t.Fatalf("failed to send JSON message: %v", err)
	}

	// ----------步骤7：节点2发送普通文本消息到节点1----------
	textMsg := &pb.GossipMessage{
		Hash:     "msg-text-01",
		FromHash: node2.NodeHash,
		PayLoad:  []byte("Hello from node2"),
	}
	err = node2.SendMessageToNode("node1", textMsg)
	if err != nil {
		t.Fatalf("failed to send text message: %v", err)
	}

	// ----------步骤8：等待消息传播----------
	time.Sleep(500 * time.Millisecond)

	// ----------步骤9：检查节点1和节点2的消息接收情况----------
	checkMessages := []string{"msg-json-01", "msg-text-01"}
	nodes := []*NodeManage.GossipNode{node1, node2}

	for _, n := range nodes {
		for _, msgHash := range checkMessages {
			states := n.Storage.GetStates(msgHash)
			if states == nil {
				n.Logger.Log(n.NodeHash, msgHash+" 的状态 | 没有记录")
				continue
			}

			received := []string{}
			notReceived := []string{}
			for neighbor, ok := range states {
				if ok {
					received = append(received, neighbor)
				} else {
					notReceived = append(notReceived, neighbor)
				}
			}

			n.Logger.Log(msgHash+" 收到的节点: "+StringSlice(received), n.NodeHash)
			n.Logger.Log(msgHash+" 未收到的节点: "+StringSlice(notReceived), n.NodeHash)
		}
	}

	// ----------步骤10：清理----------
	node1.StopGRPC()
	node2.StopGRPC()
}

// 辅助函数，把 string slice 拼成逗号分隔的字符串
func StringSlice(ss []string) string {
	if len(ss) == 0 {
		return "none"
	}
	res := ""
	for i, s := range ss {
		if i > 0 {
			res += ", "
		}
		res += s
	}
	return res
}
