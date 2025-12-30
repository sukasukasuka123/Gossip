// NodeManage/chunk_node.go
package NodeManage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/sukasukasuka123/Gossip/ConnManager"
	"github.com/sukasukasuka123/Gossip/MessageManage"
	"github.com/sukasukasuka123/Gossip/NeighborManage"
	pb "github.com/sukasukasuka123/Gossip/gossip_rpc_chunk/proto"
)

// ChunkNode 完整的 Gossip 节点
type ChunkNode struct {
	// 基本信息
	nodeHash string
	address  string
	port     int

	// 核心组件
	neighborMgr    *NeighborManage.NeighborManager
	messageManager *MessageManage.MessageManager
	grpcServer     *grpc.Server
	chunkServer    *ConnManager.GossipChunkServer

	// gRPC 客户端管理
	mu          sync.RWMutex
	grpcClients map[string]pb.GossipChunkServiceClient // "address:port" -> client
	grpcConns   map[string]*grpc.ClientConn            // "address:port" -> conn

	// 运行状态
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 统计信息
	sessionID        atomic.Int32
	messagesSent     atomic.Int64
	messagesRecved   atomic.Int64
	bytesTransferred atomic.Int64

	// 配置
	config *NodeConfig
}

// NodeConfig 节点配置
type NodeConfig struct {
	NodeHash        string                        // 节点哈希（如果为空则自动生成）
	Address         string                        // 监听地址
	Port            int                           // 监听端口
	ChunkSize       int                           // 分块大小（字节）
	FanoutCount     int                           // fanout 数量
	FanoutStrategy  NeighborManage.FanoutStrategy // fanout 策略
	WindowSize      int                           // 滑动窗口大小
	DeduplicatorTTL time.Duration                 // 去重记录保留时长
	CleanupInterval time.Duration                 // 清理间隔
	StaleDuration   time.Duration                 // 邻居失效时长
}

// DefaultNodeConfig 返回默认配置
func DefaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		NodeHash:        "",
		Address:         "0.0.0.0",
		Port:            50051,
		ChunkSize:       64 * 1024, // 64KB
		FanoutCount:     3,
		FanoutStrategy:  NeighborManage.FanoutByLatency,
		WindowSize:      15,
		DeduplicatorTTL: 10 * time.Minute,
		CleanupInterval: time.Minute,
		StaleDuration:   5 * time.Minute,
	}
}

// NewChunkNode 创建新的 ChunkNode
func NewChunkNode(config *NodeConfig) *ChunkNode {
	if config == nil {
		config = DefaultNodeConfig()
	}

	// 如果没有指定 nodeHash，自动生成
	if config.NodeHash == "" {
		config.NodeHash = generateNodeHash(config.Address, config.Port)
	}

	ctx, cancel := context.WithCancel(context.Background())

	node := &ChunkNode{
		nodeHash:    config.NodeHash,
		address:     config.Address,
		port:        config.Port,
		grpcClients: make(map[string]pb.GossipChunkServiceClient),
		grpcConns:   make(map[string]*grpc.ClientConn),
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
	}

	// 初始化组件
	node.initComponents()

	return node
}

// initComponents 初始化所有组件
func (n *ChunkNode) initComponents() {
	// 1. 创建消息管理组件
	chunker := MessageManage.NewChunker(n.config.ChunkSize)
	reassembler := MessageManage.NewReassembler()
	cache := MessageManage.NewInMemoryCache()
	deduplicator := MessageManage.NewMessageDeduplicator(n.config.DeduplicatorTTL)

	n.messageManager = MessageManage.NewMessageManager(
		chunker,
		reassembler,
		cache,
		deduplicator,
	)

	// 2. 创建邻居管理器
	n.neighborMgr = NeighborManage.NewNeighborManager(
		n.config.FanoutStrategy,
		n.config.FanoutCount,
	)

	// 3. 创建 gRPC 服务端
	n.chunkServer = ConnManager.NewGossipChunkServer(
		n.nodeHash,
		n.messageManager,
		n, // ChunkNode 实现 MessageHandler 接口
	)

	n.grpcServer = grpc.NewServer()
	pb.RegisterGossipChunkServiceServer(n.grpcServer, n.chunkServer)
}

// Start 启动节点
func (n *ChunkNode) Start() error {
	// 1. 启动 gRPC 服务器
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", n.address, n.port))
	if err != nil {
		return fmt.Errorf("启动监听失败: %w", err)
	}

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		log.Printf("[Node %s] gRPC 服务器启动在 %s:%d", n.nodeHash, n.address, n.port)
		if err := n.grpcServer.Serve(listener); err != nil {
			log.Printf("[Node %s] gRPC 服务器错误: %v", n.nodeHash, err)
		}
	}()

	// 2. 启动定期清理任务
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.cleanupRoutine()
	}()

	// 3. 启动统计日志
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.statsRoutine()
	}()

	log.Printf("[Node %s] 节点启动完成", n.nodeHash)
	return nil
}

// Stop 停止节点
func (n *ChunkNode) Stop() error {
	log.Printf("[Node %s] 正在停止节点...", n.nodeHash)

	// 1. 取消所有协程
	n.cancel()

	// 2. 停止 gRPC 服务器
	if n.grpcServer != nil {
		n.grpcServer.GracefulStop()
	}

	// 3. 关闭所有 gRPC 客户端连接
	n.mu.Lock()
	for addr, conn := range n.grpcConns {
		if err := conn.Close(); err != nil {
			log.Printf("[Node %s] 关闭连接 %s 失败: %v", n.nodeHash, addr, err)
		}
	}
	n.grpcConns = make(map[string]*grpc.ClientConn)
	n.grpcClients = make(map[string]pb.GossipChunkServiceClient)
	n.mu.Unlock()

	// 4. 等待所有协程结束
	n.wg.Wait()

	log.Printf("[Node %s] 节点已停止", n.nodeHash)
	return nil
}

// OnMessageComplete 实现 MessageHandler 接口
func (n *ChunkNode) OnMessageComplete(payloadHash string, payload []byte, senderHash string) {
	n.messagesRecved.Add(1)
	n.bytesTransferred.Add(int64(len(payload)))

	log.Printf("[Node %s] 收到完整消息: hash=%s, sender=%s, size=%d bytes",
		n.nodeHash, payloadHash, senderHash, len(payload))

	// TODO: 这里可以添加业务逻辑处理
	// 例如：通知应用层、存储消息等

	// 执行 Gossip 转发
	n.forwardMessage(payloadHash, payload, senderHash)
}

// forwardMessage 转发消息
func (n *ChunkNode) forwardMessage(payloadHash string, payload []byte, senderHash string) {
	err := n.neighborMgr.FanoutMessageWithCallback(
		payloadHash,
		senderHash,
		func(slot *ConnManager.NeighborSlot, neighborHash string) error {
			// 分块消息
			chunks := n.messageManager.GetChunker().Split(
				payload,
				payloadHash,
				n.getNextSessionID(),
				n.nodeHash,
			)

			log.Printf("[Node %s] 转发消息 %s 给邻居 %s，共 %d 个 chunks",
				n.nodeHash, payloadHash, neighborHash, len(chunks))

			// 发送给该邻居
			slot.ReadySendMsg(chunks)
			return nil
		},
	)

	if err != nil {
		log.Printf("[Node %s] 转发消息失败: %v", n.nodeHash, err)
	}
}

// BroadcastMessage 广播消息
func (n *ChunkNode) BroadcastMessage(payload []byte) string {
	payloadHash := n.computeHash(payload)

	n.messagesSent.Add(1)
	n.bytesTransferred.Add(int64(len(payload)))

	log.Printf("[Node %s] 广播消息: hash=%s, size=%d bytes",
		n.nodeHash, payloadHash, len(payload))

	// 标记为已见（避免接收到回传时重复处理）
	n.messageManager.GetDeduplicator().MarkAsSeen(payloadHash)

	// 转发消息
	n.forwardMessage(payloadHash, payload, n.nodeHash)

	return payloadHash
}

// ConnectToNeighbor 连接到邻居节点
func (n *ChunkNode) ConnectToNeighbor(address string, port int) error {
	neighborAddr := fmt.Sprintf("%s:%d", address, port)
	neighborHash := generateNodeHash(address, port)

	// 检查是否已经连接
	if n.neighborMgr.HasNeighbor(neighborHash) {
		return fmt.Errorf("已经连接到邻居 %s", neighborAddr)
	}

	// 1. 创建 gRPC 客户端
	client, err := n.getOrCreateClient(neighborAddr)
	if err != nil {
		return fmt.Errorf("创建客户端失败: %w", err)
	}

	// 2. 创建流
	stream, err := client.PushChunks(n.ctx)
	if err != nil {
		return fmt.Errorf("创建流失败: %w", err)
	}

	// 3. 创建 NeighborSlot
	slot := ConnManager.NewNeighborSlot(neighborHash, stream, n.config.WindowSize, n.ctx)
	slot.StartRecvAck()
	slot.StartHeartbeat()

	// 4. 添加到邻居管理器
	node := &NeighborManage.NeighborNode{
		NodeHash: neighborHash,
		Address:  address,
		Port:     port,
		Ping:     10 * time.Millisecond, // 初始值，后续可以通过心跳更新
	}
	n.neighborMgr.AddNeighbor(node, slot)

	log.Printf("[Node %s] 成功连接到邻居: %s (hash=%s)",
		n.nodeHash, neighborAddr, neighborHash)

	return nil
}

// DisconnectNeighbor 断开邻居连接
func (n *ChunkNode) DisconnectNeighbor(neighborHash string) error {
	neighbor, exists := n.neighborMgr.GetNeighbor(neighborHash)
	if !exists {
		return fmt.Errorf("邻居不存在: %s", neighborHash)
	}

	// 从邻居管理器移除
	n.neighborMgr.RemoveNeighbor(neighborHash)

	neighborAddr := fmt.Sprintf("%s:%d", neighbor.Address, neighbor.Port)
	log.Printf("[Node %s] 断开邻居: %s", n.nodeHash, neighborAddr)

	return nil
}

// getOrCreateClient 获取或创建 gRPC 客户端
func (n *ChunkNode) getOrCreateClient(address string) (pb.GossipChunkServiceClient, error) {
	n.mu.RLock()
	if client, exists := n.grpcClients[address]; exists {
		n.mu.RUnlock()
		return client, nil
	}
	n.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	// 双重检查
	if client, exists := n.grpcClients[address]; exists {
		return client, nil
	}

	// 创建新连接
	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	client := pb.NewGossipChunkServiceClient(conn)
	n.grpcClients[address] = client
	n.grpcConns[address] = conn

	return client, nil
}

// cleanupRoutine 定期清理任务
func (n *ChunkNode) cleanupRoutine() {
	ticker := time.NewTicker(n.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			// 清理失效邻居
			cleaned := n.neighborMgr.CleanupStaleNeighbors(n.config.StaleDuration)
			if cleaned > 0 {
				log.Printf("[Node %s] 清理了 %d 个失效邻居", n.nodeHash, cleaned)
			}
		}
	}
}

// statsRoutine 统计日志
func (n *ChunkNode) statsRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			stats := n.GetStats()
			log.Printf("[Node %s] 统计: 邻居=%d, 已发=%d, 已收=%d, 传输=%d bytes, 缓存=%d, 去重=%d",
				n.nodeHash,
				stats["active_neighbors"],
				stats["messages_sent"],
				stats["messages_received"],
				stats["bytes_transferred"],
				stats["cache_size"],
				stats["dedup_records"],
			)
		}
	}
}

// GetStats 获取节点统计信息
func (n *ChunkNode) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"node_hash":         n.nodeHash,
		"address":           fmt.Sprintf("%s:%d", n.address, n.port),
		"active_neighbors":  n.neighborMgr.ActiveNeighborCount(),
		"messages_sent":     n.messagesSent.Load(),
		"messages_received": n.messagesRecved.Load(),
		"bytes_transferred": n.bytesTransferred.Load(),
		"cache_size":        n.messageManager.GetCache(),
		"dedup_records":     n.messageManager.GetDeduplicator().Count(),
		"fanout_stats":      n.neighborMgr.GetFanoutStats(),
	}
}

// GetNodeHash 获取节点哈希
func (n *ChunkNode) GetNodeHash() string {
	return n.nodeHash
}

// GetNeighbors 获取所有邻居
func (n *ChunkNode) GetNeighbors() []*NeighborManage.NeighborNode {
	return n.neighborMgr.GetAllNeighbors()
}

// getNextSessionID 获取下一个 session ID
func (n *ChunkNode) getNextSessionID() int32 {
	return n.sessionID.Add(1)
}

// computeHash 计算哈希
func (n *ChunkNode) computeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// generateNodeHash 生成节点哈希
func generateNodeHash(address string, port int) string {
	data := fmt.Sprintf("%s:%d:%d", address, port, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("node_%x", hash[:8])
}
