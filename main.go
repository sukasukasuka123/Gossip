package main

import (
	"Gossip/MessageManage"
	"Gossip/NodeManage"
	"Gossip/NodeManage/Logger"
	"Gossip/NodeManage/Router"
	"Gossip/NodeManage/Storage"
	"Gossip/NodeManage/TransportMessage"
	"fmt" // 引入 fmt 用于字符串格式化

	// "Gossip/Util/HeartBeat"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 定义节点数量
const numNodes = 10

func main() {
	// 用于存储所有节点的哈希 -> 节点实例
	nodes := make(map[string]*NodeManage.GossipNode[string])
	// 用于存储所有节点的哈希 -> 完整 HTTP 端点 URL
	endpoints := make(map[string]string)

	// 1. 循环创建 10 个节点和它们的 Logger
	for i := 1; i <= numNodes; i++ {
		nodeID := fmt.Sprintf("node%d.txt", i)
		port := 8080 + i
		addr := fmt.Sprintf(":%d", port)               // HTTP 监听地址 (e.g., ":8081")
		url := fmt.Sprintf("http://localhost%s", addr) // 完整 URL (e.g., "http://localhost:8081")

		// 初始化 Logger (取值以匹配 GossipNode 字段)
		nodeLogger := *Logger.NewLogger()

		// 创建节点。nodeID 将作为 NodeHash，同时也是日志文件名。
		node := createNode[string](nodeID, addr, nodeLogger, "")

		nodes[nodeID] = node
		endpoints[nodeID] = url
	}

	// 2. 设置邻居关系 (创建全连接网络：每个节点都连接其他所有节点)
	for _, node := range nodes {
		neighbors := make(map[string]string)
		for hash, ep := range endpoints {
			if hash != node.NodeHash {
				neighbors[hash] = ep
			}
		}
		addNeighbors(node, neighbors)
	}

	// 3. 启动 HTTP 服务
	for _, node := range nodes {
		// startServer 需要传入监听地址 (addr)，它存储在 node.Neighbors[node.NodeHash] 中
		startServer(node, node.Neighbors[node.NodeHash])
	}

	// 等待服务启动
	time.Sleep(2 * time.Second)

	// 4. 传输 4 个消息，分别从 node1, node2, node3, node4 启动
	sourceHashes := []string{"node1.txt", "node2.txt", "node3.txt", "node4.txt"}

	for i, sourceHash := range sourceHashes {
		sourceNode := nodes[sourceHash]
		msgHash := fmt.Sprintf("test-message-%d", i+1)

		testMsg := MessageManage.GossipMessage[string]{
			Hash:     msgHash,
			FromHash: "", // 源发消息
			Payload:  fmt.Sprintf("Payload for %s, from %s", msgHash, sourceHash),
		}

		data, err := json.Marshal(testMsg)
		if err != nil {
			log.Fatalf("消息 %s 序列化失败: %v", msgHash, err)
		}

		// 目标 URL 是源节点自身的完整 URL
		targetURL := endpoints[sourceHash]

		err = sourceNode.Transport.SendMessage(context.Background(), targetURL, data)
		if err != nil {
			log.Printf("发送消息 %s 失败 from %s: %v", msgHash, sourceHash, err)
		}
		log.Printf("测试消息 %s 已发送，源节点: %s", msgHash, sourceHash)
	}

	// 最终等待
	log.Println("等待10秒以便消息传播...")
	time.Sleep(10 * time.Second)
	log.Println("测试完成，请检查 node1.txt 至 node10.txt 的日志文件")
}

// ====================================================================
// 辅助函数 (保持不变，但 StartServer 使用了 NodeHash 作为地址获取 URL)
// ====================================================================

func createNode[T any](
	nodeID string,
	endpoint string,
	customLogger Logger.Logger,
	payload T,
) *NodeManage.GossipNode[T] {
	storage := Storage.NewLocalStorage(4, 10*time.Minute)
	transport := TransportMessage.NewHttpTransport()
	router := Router.NewFanoutRouter()

	// 使用 NewGossipNode 构造函数
	node := NodeManage.NewGossipNode[T](
		nodeID,
		endpoint,
		payload,
		storage,
		transport,
		router,
		customLogger,
	)
	// 将自己的监听地址也添加到 Neighbors 中，方便 startServer 调用
	node.Neighbors[nodeID] = endpoint
	return node
}

func addNeighbors[T any](node *NodeManage.GossipNode[T], neighbors map[string]string) {
	for hash, endpoint := range neighbors {
		node.AddNeighbor(hash, endpoint)
	}
}

func startServer[T any](node *NodeManage.GossipNode[T], addr string) {
	r := gin.Default()
	node.RegisterRoutes(r)
	go func() {
		if err := http.ListenAndServe(addr, r); err != nil && err != http.ErrServerClosed {
			logText := fmt.Sprintf("节点 %s 启动失败: %v", node.NodeHash, err)
			node.Logger.Log(logText, node.NodeHash)
		}
	}()
	logText := fmt.Sprintf("节点 %s 已启动，监听地址: %s", node.NodeHash, addr)
	node.Logger.Log(logText, node.NodeHash)
}
