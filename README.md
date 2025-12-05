# Gossip 协议系统说明文档（基于 gRPC 的新版）

一个基于 **gRPC 传输层** 的分布式 Gossip 协议系统，实现节点间的去中心化消息广播、重复抑制、基于 ACK 的确认与可靠传播、以及按拓扑结构自适应的 TTL 状态管理。

本版 README 已完全更新为 **gRPC 版本**，删除旧的 HTTP/gin 相关内容，文档重点转向 gRPC 服务接口、客户端工厂、节点模型更新后的调用方式和参数说明。

---

## 系统概述

系统采用 Gossip（谣言式扩散）机制进行消息传播：节点收到消息后去重、确认、再向部分邻居广播，最终消息可在网络中快速“泛洪”。新版本将底层传输从 HTTP/gin 替换为 **Protocol Buffers + gRPC**，实现更高吞吐、更低延迟、支持连接复用与双向通信，为生产级调度提供基础。

核心变化：

* 使用 protobuf 定义 Gossip RPC 协议。
* 每个节点启动一个 gRPC 服务器，注册 Gossip 服务。
* 通过 **GossipClientFactory** 实现可复用的 gRPC 客户端连接（Dial 缓存）。
* 所有消息、ACK 均通过 RPC 方法 `PutMessageToClient` 发送。

---

## 模块结构

### 1. protobuf 层（proto/）

定义了 Gossip RPC 协议：

* GossipMessage：包含 hash、来源节点、payload。
* GossipAck：确认消息。
* Gossip 服务：提供 `PutMessageToClient` RPC。

生成文件：

* `gossip_rpc.pb.go`
* `gossip_rpc_grpc.pb.go`

这些文件为服务器和客户端提供 Go API。

---

### 2. 节点管理（NodeManage/）

新版 `GossipNode` 已非泛型，结构被重写以使用 gRPC。

核心内容：

* 嵌入 `pb.UnimplementedGossipServer`，作为 RPC 服务端。
* 维护邻居映射（neighborHash → endpoint）。
* 内置 TTL 状态机、重复检测、邻居 ACK 统计。
* 使用 GossipClientFactory 进行 RPC 调用。

#### 关键方法

**StartGRPCServer(port string)**

* 启动 gRPC 服务，监听端口。
* 注册 Gossip 服务实现。

**PutMessageToClient(context, *pb.GossipMessage)**

* RPC 接收入口。
* 完成去重、初始化状态、记录 ACK、广播给 fanout 邻居。

**AddNeighbor(nodeHash, endpoint)**

* 动态添加邻居端点（如："127.0.0.1:9001"）。
* 更新 TTL 限制参数。

内部广播全部通过：

**broadcastToTargets(targets []string, msg *pb.GossipMessage)**

* 为每个 target 取出客户端
* 构造 context with timeout
* 执行异步 RPC

---

### 3. GossipClientFactory（可复用的客户端连接池）

文件：`GossipClientFactory/GossipClientFactory.go`

用于缓存 Dial 的 grpc.ClientConn：

* 相同 endpoint 的连接不会重复拨号
* 节省连接建立成本
* 提供 Release() 方法统一关闭
* 提供 ContextWithTimeout 创建外发 RPC 的超时上下文

使用方式示例：

```
cli := factory.GetClient(endpoint)
resp, err := cli.PutMessageToClient(ctx, msg)
```

---

### 4. 存储与 TTL

仍使用之前的 LocalStorage 管理：

* 消息已见（seen）缓存
* 邻居 ACK 状态表
* 自适应 TTL（2～64 秒范围）
* 长 TTL 缓存 10 分钟
* 周期衰减机制维持稳定内存占用

TTL 行为不因迁移到 gRPC 而改变。

---

## gRPC API 定义说明

### RPC：PutMessageToClient

```
rpc PutMessageToClient(GossipMessage) returns (GossipAck)
```

作用：

* 接收消息 / 去重
* 初始化状态
* 返回 ACK
* fanout 广播

入参：pb.GossipMessage

* hash：消息唯一 ID
* fromHash：来源节点
* payload：字符串载荷

返回：pb.GossipAck

* hash：同一消息 hash
* from：接收方节点 hash

---

## GossipNode 的主要使用方法

### 1. 创建节点

```go
node := NewGossipNode(
    selfHash,
    endpoint,
    storage,
    router,
    logger,
    clientFactory,
)
```

### 2. 启动 gRPC 服务

```go
node.StartGRPCServer(":9001")
```

### 3. 添加邻居

```go
node.AddNeighbor("node2", "127.0.0.1:9002")
```

### 4. 主动发消息（本地触发）

```go
msg := &pb.GossipMessage{Hash: "123", FromHash: node.Hash, Payload: "hello"}
node.PutMessageToClient(context.Background(), msg)
```

---

## main.go 测试逻辑（新版）

新版示例测试创建 **20 个节点**，端口从 6001 到 6020，以 **环形拓扑** 相互连接。

流程：

1. 初始化统一 storage/router/logger
2. 创建 20 个 GossipNode
3. 每个节点启动 gRPC 服务
4. 构建环形邻居（i → i+1）
5. 从每个节点发送 3 条测试消息
6. 等待传播完成后退出

**不再使用任何 HTTP / gin / JSON**。

---

## 当前文件结构

```
Gossip/
├── gossip_rpc/              # 生成的 protobuf RPC 文件
├── proto/                   # .proto 文件
├── NodeManage/              # gRPC 化后的 GossipNode
│   ├── gossip_grpc.go       # RPC 处理、广播逻辑
│   ├── AddNeighbor.go
│   ├── node_model.go
│   └── ...
├── GossipClientFactory/     # gRPC 客户端工厂
├── go.mod / go.sum
└── main.go                  # gRPC 测试入口
```

---

## 当前限制

* 无 TLS / 认证
* 无连接心跳或健康检查
* 固定 fanout=3
* 无持久化状态
* 单向 RPC（并未使用流式）

---

## 未来改进方向（基于 gRPC 的路线）

* 增加 TLS、token 或 mTLS 身份校验
* 引入 gRPC bidirectional streaming，支持批量广播
* 添加健康检查与失败节点剔除机制
* 动态调节 fanout
* 增加持久化状态实现崩溃恢复
* 增加监控、指标、可视化工具

---

## 总结

新版系统保留了原 Gossip 算法的核心机制（去重、ACK、fanout、TTL），同时将网络层彻底替换为 gRPC，实现更快、更稳定、可扩展的传输基础设施。整个项目的结构更清晰，节点启动、邻居维护、消息处理都围绕 gRPC API 展开，更适合继续扩展成真实的分布式系统原型。
