package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sukasukasuka123/Gossip/NeighborManage"
	"github.com/sukasukasuka123/Gossip/NodeManage"
)

// CLI Gossip 节点命令行接口
type CLI struct {
	node   *NodeManage.ChunkNode
	reader *bufio.Reader
}

// NewCLI 创建新的 CLI 实例
func NewCLI(node *NodeManage.ChunkNode) *CLI {
	return &CLI{
		node:   node,
		reader: bufio.NewReader(os.Stdin),
	}
}

// Run 运行 CLI 交互循环
func (c *CLI) Run() {
	fmt.Println("=== Gossip Node CLI ===")
	fmt.Printf("Node Hash: %s\n", c.node.GetNodeHash())
	fmt.Println("Type 'help' for available commands")
	fmt.Println()

	for {
		fmt.Print("> ")
		input, err := c.reader.ReadString('\n')
		if err != nil {
			fmt.Printf("读取输入错误: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]
		args := parts[1:]

		if err := c.executeCommand(command, args); err != nil {
			fmt.Printf("错误: %v\n", err)
		}
	}
}

// executeCommand 执行命令
func (c *CLI) executeCommand(command string, args []string) error {
	switch command {
	case "help", "h":
		c.printHelp()
	case "stop":
		return c.stopNode()
	case "connect", "conn":
		return c.connectNeighbor(args)
	case "disconnect", "disc":
		return c.disconnectNeighbor(args)
	case "broadcast", "bc":
		return c.broadcastMessage(args)
	case "neighbors", "nb":
		c.listNeighbors()
	case "stats", "st":
		c.showStats()
	case "info":
		c.showNodeInfo()
	case "exit", "quit", "q":
		fmt.Println("退出 CLI...")
		os.Exit(0)
	default:
		return fmt.Errorf("未知命令: %s (输入 'help' 查看可用命令)", command)
	}
	return nil
}

// printHelp 打印帮助信息
func (c *CLI) printHelp() {
	help := `
可用命令:
  help, h                          - 显示此帮助信息
  stop                             - 停止节点
  connect <address> <port>         - 连接到邻居节点
  disconnect <node_hash>           - 断开邻居连接
  broadcast <message>              - 广播消息
  neighbors, nb                    - 列出所有邻居
  stats, st                        - 显示节点统计信息
  info                             - 显示节点基本信息
  exit, quit, q                    - 退出程序

示例:
  connect 127.0.0.1 50052          - 连接到本地 50052 端口的节点
  disconnect node_abc123           - 断开哈希为 node_abc123 的邻居
  broadcast Hello World            - 广播 "Hello World" 消息
`
	fmt.Println(help)
}

// startNode 启动节点
func (c *CLI) startNode() error {
	fmt.Println("正在启动节点...")
	if err := c.node.Start(); err != nil {
		return fmt.Errorf("启动节点失败: %w", err)
	}
	fmt.Println("节点启动成功")
	return nil
}

// stopNode 停止节点
func (c *CLI) stopNode() error {
	fmt.Println("正在停止节点...")
	if err := c.node.Stop(); err != nil {
		return fmt.Errorf("停止节点失败: %w", err)
	}
	fmt.Println("节点停止成功")
	return nil
}

// connectNeighbor 连接邻居节点
func (c *CLI) connectNeighbor(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("用法: connect <address> <port>")
	}

	address := args[0]
	port, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("无效的端口号: %s", args[1])
	}

	fmt.Printf("正在连接到 %s:%d...\n", address, port)
	if err := c.node.ConnectToNeighbor(address, port); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	fmt.Printf("成功连接到 %s:%d\n", address, port)
	return nil
}

// disconnectNeighbor 断开邻居连接
func (c *CLI) disconnectNeighbor(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: disconnect <node_hash>")
	}

	nodeHash := args[0]
	fmt.Printf("正在断开邻居 %s...\n", nodeHash)

	if err := c.node.DisconnectNeighbor(nodeHash); err != nil {
		return fmt.Errorf("断开连接失败: %w", err)
	}

	fmt.Printf("成功断开邻居 %s\n", nodeHash)
	return nil
}

// broadcastMessage 广播消息
func (c *CLI) broadcastMessage(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: broadcast <message>")
	}

	message := strings.Join(args, " ")
	fmt.Printf("正在广播消息: %s\n", message)

	payloadHash := c.node.BroadcastMessage([]byte(message))
	fmt.Printf("消息已广播，hash: %s\n", payloadHash)

	return nil
}

// listNeighbors 列出所有邻居
func (c *CLI) listNeighbors() {
	neighbors := c.node.GetNeighbors()

	if len(neighbors) == 0 {
		fmt.Println("当前没有邻居节点")
		return
	}

	fmt.Printf("\n当前邻居节点 (共 %d 个):\n", len(neighbors))
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-20s %-25s %-10s\n", "Node Hash", "Address", "Latency")
	fmt.Println(strings.Repeat("-", 80))

	for _, neighbor := range neighbors {
		fmt.Printf("%-20s %-25s %-10s\n",
			neighbor.NodeHash,
			fmt.Sprintf("%s:%d", neighbor.Address, neighbor.Port),
			neighbor.Ping,
		)
	}
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println()
}

// showStats 显示统计信息
func (c *CLI) showStats() {
	stats := c.node.GetStats()

	fmt.Println("\n=== 节点统计信息 ===")
	fmt.Println(strings.Repeat("-", 60))

	fmt.Printf("节点哈希:         %s\n", stats["node_hash"])
	fmt.Printf("监听地址:         %s\n", stats["address"])
	fmt.Printf("活跃邻居数:       %d\n", stats["active_neighbors"])
	fmt.Printf("已发送消息数:     %d\n", stats["messages_sent"])
	fmt.Printf("已接收消息数:     %d\n", stats["messages_received"])
	fmt.Printf("总传输字节数:     %d bytes\n", stats["bytes_transferred"])
	fmt.Printf("缓存大小:         %v\n", stats["cache_size"])
	fmt.Printf("去重记录数:       %d\n", stats["dedup_records"])

	if fanoutStats, ok := stats["fanout_stats"].(map[string]interface{}); ok {
		fmt.Println("\nFanout 统计:")
		for k, v := range fanoutStats {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()
}

// showNodeInfo 显示节点基本信息
func (c *CLI) showNodeInfo() {
	stats := c.node.GetStats()
	neighbors := c.node.GetNeighbors()

	fmt.Println("\n=== 节点信息 ===")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("节点哈希:     %s\n", stats["node_hash"])
	fmt.Printf("监听地址:     %s\n", stats["address"])
	fmt.Printf("邻居数量:     %d\n", len(neighbors))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()
}

// RunWithConfig 使用配置启动 CLI
func RunWithConfig(config *NodeManage.NodeConfig) {
	// 创建节点
	node := NodeManage.NewChunkNode(config)

	// 启动节点
	if err := node.Start(); err != nil {
		fmt.Printf("启动节点失败: %v\n", err)
		os.Exit(1)
	}

	// 创建并运行 CLI
	cli := NewCLI(node)
	cli.Run()
}

// QuickStart 快速启动（使用默认配置）
func QuickStart(address string, port int) {
	config := NodeManage.DefaultNodeConfig()
	config.Address = address
	config.Port = port

	RunWithConfig(config)
}

// StartWithCustomConfig 使用自定义配置启动
func StartWithCustomConfig(
	address string,
	port int,
	chunkSize int,
	fanoutCount int,
	fanoutStrategy NeighborManage.FanoutStrategy,
) {
	config := NodeManage.DefaultNodeConfig()
	config.Address = address
	config.Port = port
	config.ChunkSize = chunkSize
	config.FanoutCount = fanoutCount
	config.FanoutStrategy = fanoutStrategy

	RunWithConfig(config)
}

// BatchConnectNeighbors 批量连接邻居节点
func (c *CLI) BatchConnectNeighbors(neighbors []struct {
	Address string
	Port    int
}) error {
	fmt.Printf("正在批量连接 %d 个邻居节点...\n", len(neighbors))

	successCount := 0
	for i, neighbor := range neighbors {
		fmt.Printf("[%d/%d] 连接 %s:%d...", i+1, len(neighbors), neighbor.Address, neighbor.Port)

		if err := c.node.ConnectToNeighbor(neighbor.Address, neighbor.Port); err != nil {
			fmt.Printf(" 失败: %v\n", err)
		} else {
			fmt.Println(" 成功")
			successCount++
		}

		// 避免连接过快
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("\n批量连接完成: 成功 %d/%d\n", successCount, len(neighbors))
	return nil
}

// WatchStats 持续监控统计信息
func (c *CLI) WatchStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Println("开始监控统计信息 (按 Ctrl+C 退出)...")
	fmt.Println()

	for range ticker.C {
		stats := c.node.GetStats()
		fmt.Printf("[%s] 邻居=%d, 发送=%d, 接收=%d, 传输=%d bytes\n",
			time.Now().Format("15:04:05"),
			stats["active_neighbors"],
			stats["messages_sent"],
			stats["messages_received"],
			stats["bytes_transferred"],
		)
	}
}
