package main

import (
	"Gossip/NodeManage"
	"Gossip/NodeManage/Logger"
	"Gossip/NodeManage/Router"
	"Gossip/NodeManage/Storage"
	pb "Gossip/gossip_rpc/proto"
	"context"
	"fmt"
	"time"
)

// 测试函数：20个节点，每个节点发送3条消息
func TestGossipNetwork() {
	nodeCount := 20
	nodes := make([]*NodeManage.GossipNode, nodeCount)

	store := Storage.NewLocalStorage(200, 5*time.Minute)
	router := Router.NewFanoutRouter()
	log := Logger.NewLogger()

	// 1. 初始化节点并启动 gRPC server
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("node%02d", i+1)
		nodes[i] = NodeManage.NewGossipNode(nodeID, store, router, log, 5*time.Second)

		port := 50050 + i
		endpoint := fmt.Sprintf(":%d", port)
		nodes[i].StartGRPCServer(endpoint)

		fmt.Printf("[Init] %s started at %s\n", nodeID, endpoint)
	}

	// 2. 注册邻居（环形结构，每个节点连接下一个节点）
	for i := 0; i < nodeCount; i++ {
		next := (i + 1) % nodeCount
		nextID := nodes[next].NodeHash
		nextEP := fmt.Sprintf("localhost:%d", 50050+next)

		nodes[i].AddNeighbor(nextID, nextEP)
		fmt.Printf("[Neighbor] %s -> %s (%s)\n", nodes[i].NodeHash, nextID, nextEP)
	}

	time.Sleep(1 * time.Second) // 等待所有 gRPC 启动完毕

	// 3. 每个节点发送3条消息
	for i, node := range nodes {
		for j := 1; j <= 3; j++ {
			msg := &pb.GossipMessage{
				Hash:     fmt.Sprintf("msg-%02d-%d", i+1, j),
				FromHash: node.NodeHash,
				PayLoad:  []byte(fmt.Sprintf("Message %d from %s", j, node.NodeHash)),
			}

			go func(n *NodeManage.GossipNode, m *pb.GossipMessage) {
				ack, err := n.PutMessageToClient(context.Background(), m)
				if err != nil {
					fmt.Printf("[%s] Send failed: %v\n", n.NodeHash, err)
				} else {
					fmt.Printf("[%s] ACK received from: %s for %s\n", n.NodeHash, ack.FromHash, m.Hash)
				}
			}(node, msg)
		}
	}

	fmt.Println("All messages dispatched. Waiting for gossip to propagate...")

	// 等待所有消息广播完成
	time.Sleep(3 * time.Minute)
	fmt.Println("Test completed.")
}

func main() {
	TestGossipNetwork()
}
