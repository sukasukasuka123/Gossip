package main

import (
	"Gossip/MessageManage"
	"Gossip/NodeManage"
	"Gossip/NodeManage/Router"
	"Gossip/NodeManage/Storage"
	"Gossip/NodeManage/TransportMessage"
	"Gossip/Util/HeartBeat"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// 明确指定泛型类型为string（根据消息payload类型确定）
	node1 := createNode[string]("node1", ":8081", "")
	node2 := createNode[string]("node2", ":8082", "")
	node3 := createNode[string]("node3", ":8083", "")

	// 设置节点间的邻居关系
	addNeighbors(node1, map[string]string{
		node2.NodeHash: "http://localhost:8082",
		node3.NodeHash: "http://localhost:8083",
	})
	addNeighbors(node2, map[string]string{
		node1.NodeHash: "http://localhost:8081",
		node3.NodeHash: "http://localhost:8083",
	})
	addNeighbors(node3, map[string]string{
		node1.NodeHash: "http://localhost:8081",
		node2.NodeHash: "http://localhost:8082",
	})

	// 启动HTTP服务
	startServer(node1, ":8081")
	startServer(node2, ":8082")
	startServer(node3, ":8083")

	// 启动心跳检测
	go HeartBeat.Heartbeat("http://localhost:8081/RecieveMessage")
	go HeartBeat.Heartbeat("http://localhost:8082/RecieveMessage")
	go HeartBeat.Heartbeat("http://localhost:8083/RecieveMessage")

	// 等待服务启动
	time.Sleep(2 * time.Second)

	// 从node1发送测试消息
	testMsg := MessageManage.GossipMessage[string]{
		Hash:    "test-message-123",
		Payload: "Hello Gossip Network!",
	}
	data, err := json.Marshal(testMsg)
	if err != nil {
		log.Fatalf("消息序列化失败: %v", err)
	}
	// 发送消息到自身节点进行传播
	err = node1.Transport.SendMessage(context.Background(),
		"http://localhost:8081", data)
	if err != nil {
		log.Printf("发送消息失败: %v", err)
	}

	// 等待消息传播
	time.Sleep(10 * time.Second)
	log.Println("测试完成，查看节点日志验证消息传播情况")
}

// 创建节点辅助函数（明确泛型参数）
func createNode[T any](nodeID, endpoint string, payload T) *NodeManage.GossipNode[T] {
	storage := Storage.NewLocalStorage()
	transport := TransportMessage.NewHttpTransport()
	router := Router.NewFanoutRouter()
	// 正确传递payload参数（与T类型匹配）
	return NodeManage.NewGossipNode[T](
		nodeID,
		endpoint,
		payload, // 这里使用传入的payload参数，与T类型一致
		storage,
		transport,
		router,
	)
}

// 添加邻居辅助函数
func addNeighbors[T any](node *NodeManage.GossipNode[T], neighbors map[string]string) {
	for hash, endpoint := range neighbors {
		node.Neighbors[hash] = endpoint
		node.Cost[hash] = 1.0 // 设置默认通信代价
	}
}

// 启动HTTP服务器
func startServer[T any](node *NodeManage.GossipNode[T], addr string) {
	r := gin.Default()
	node.RegisterRoutes(r)
	go func() {
		if err := http.ListenAndServe(addr, r); err != nil && err != http.ErrServerClosed {
			log.Printf("节点 %s 启动失败: %v", node.NodeHash, err)
		}
	}()
	log.Printf("节点 %s 已启动，监听地址: %s", node.NodeHash, addr)
}
