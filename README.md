# Gossip 多节点双流系统使用说明（含大消息压测示例）

本项目实现了一套基于 **gRPC 双向流（MessageStream + AckStream）** 的 Gossip 通信系统，支持：

* 多节点并发通信
* 大消息（500KB+）稳定传输
* ACK 收敛与状态追踪
* 高并发基准测试（benchmark）

本文档说明：

1. 一个 Gossip 节点是如何初始化的
2. 多节点如何建立邻居关系
3. 如何进行多节点大消息压测

---

## 一、节点初始化流程

### 1️⃣ 节点的核心组成

一个 `DoubleStreamNode` 在初始化时包含以下核心组件：

* **NodeHash**：节点唯一标识
* **gRPC Server**：用于接收 MessageStream / AckStream
* **NeighborManager**：管理邻居节点与双流连接
* **MessageManager**：负责消息状态、ACK 路由与完成判定
* **DoubleStreamFactory**：复用与管理 gRPC 双流连接
* **Storage**：记录消息发送 / ACK 状态（带 TTL）

---

### 2️⃣ 默认节点构造函数示例

以下函数展示了一个**最小但完整**的 Gossip 节点初始化流程：

```go
func newDefaultNode(
	factory *GossipStreamFactory.DoubleStreamFactory,
	port string,
	storageSlots int64,
	storageTTL time.Duration,
) (*NodeManage.DoubleStreamNode, string) {

	id := nextNodeID()
	nodeHash := fmt.Sprintf("node-%d", id)

	// 邻居存储（内存实现）
	store := NeighborManage.NewMemoryNeighborStore()

	// 日志、路由器
	logger := Logger.NewLogger()
	router := Router.NewFanoutRouter()

	// 消息状态存储（用于 ACK 收敛）
	smgr := StorageManage.NewStorageManage(
		Storage.NewLocalStorage(storageSlots, storageTTL),
	)

	// 创建节点
	node := NodeManage.NewDoubleStreamNode(
		nodeHash,
		router,
		logger,
		store,
		factory,
		smgr,
	)

	// 启动 gRPC Server
	if err := node.StartGRPCServer(port); err != nil {
		panic(fmt.Sprintf("failed to start gRPC server on %s: %v", port, err))
	}

	return node, nodeHash
}
```

📌 **要点说明**

* `DoubleStreamFactory` **应全局共享**，用于复用 gRPC 连接
* 每个节点监听一个独立端口
* 节点启动后即可接受其他节点的流式连接

---

## 二、节点之间建立邻居关系

### 1️⃣ 邻居模型

每个节点通过 `NeighborManager` 管理邻居，邻居信息包括：

* `NodeHash`：邻居节点 ID
* `Endpoint`：gRPC 地址
* `Online`：是否在线

---

### 2️⃣ 节点互连示例（全互连）

以下代码展示了 **N 个节点之间建立全互连 Gossip 网络**：

```go
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
```

📌 **行为说明**

* 每次 `ConnectToNeighbor`：

  * 创建（或复用）到目标节点的双流连接
  * 自动绑定 MessageStream 接收与 ACK 写入协程
* 连接建立后，节点即可直接调用 `SendMessage`

---

## 三、多节点大消息压测（Benchmark）

### 1️⃣ Benchmark 目标

该 Benchmark 用于验证：

* 多节点（≥3）
* 大消息（≥500KB）
* 高并发发送
* ACK 是否完整收敛
* 系统在高负载下是否稳定

---

### 2️⃣ Benchmark 参数说明

```go
const (
	nodeCount       = 4          // 节点数量（>=3）
	messagesPerPeer = 30         // 每个节点给每个邻居发送的消息数
	payloadSize     = 512 * 1024 // 单条消息大小（512KB）
	basePort        = 51000
	storageSlots    = 200
	storageTTL      = 120 * time.Second
)
```

实际消息总数为：

```
nodeCount × (nodeCount - 1) × messagesPerPeer
```

---

### 3️⃣ Benchmark 核心逻辑说明

#### 🔹 消息发送阶段

* 每个节点并发向所有邻居发送消息
* 对单个邻居的发送是**串行的**
* 对不同邻居是**并行的**

```go
for k := 0; k < messagesPerPeer; k++ {
	msg := &pb.GossipMessage{
		Hash:     msgHash,
		FromHash: senderHash,
		PayLoad:  largePayload,
	}

	if err := sender.SendMessage(receiverHash, msg); err != nil {
		b.Errorf("send failed: %v", err)
	}
}
```

---

#### 🔹 ACK 收敛阶段

* 所有节点监听 `MessageManager.CompleteChan`
* 每收到一个完整 ACK 即计数
* 当 ACK 数达到预期值时结束 benchmark

```go
case <-node.MM.CompleteChan:
	if atomic.AddInt32(&ackReceived, 1) >= int32(totalMessages) {
		cancel()
		return
	}
```

---

### 4️⃣ Benchmark 成功条件

```go
if final < int32(totalMessages) {
	b.Fatalf("ACK incomplete: received %d / %d", final, totalMessages)
}
```

只有在 **所有消息的 ACK 都成功收敛** 时，Benchmark 才算通过。

---

## 四、Benchmark 结果解读（示例）

```
BenchmarkMultiNodeLargeMessage-8
3        2665671600 ns/op
         73619736 B/op
         12086 allocs/op
```

含义：

* 一次完整多节点 Gossip 回合耗时约 **2.6 秒**
* 期间堆分配约 **70MB**
* 系统在高负载下 **无死锁、无丢 ACK、无 gRPC 断流**

📌 该 Benchmark 测量的是**系统整体稳定性与吞吐能力**，而非单条消息延迟。

---

## 五、总结

* 本系统采用 **双流（Message + ACK）** 模型，避免 ACK 阻塞数据流
* 支持大消息、高并发、多节点 Gossip
* Benchmark 验证了在高负载下系统行为是 **可预测、可收敛、可关闭的**
* 后续可在此基础上扩展：

  * ACK 合并
  * Payload 零拷贝
  * Gossip 分层（metadata / data plane）

---

