package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sukasukasuka123/Gossip/NodeManage"
	"github.com/sukasukasuka123/Gossip/cli"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Gossip Node 启动向导 ===")
	fmt.Println("直接回车将使用默认配置")
	fmt.Println()

	cfg := NodeManage.DefaultNodeConfig()

	// Address
	fmt.Printf("监听地址 [%s]: ", cfg.Address)
	if v := readLine(reader); v != "" {
		cfg.Address = v
	}

	// Port
	fmt.Printf("监听端口 [%d]: ", cfg.Port)
	if v := readLine(reader); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}

	// ChunkSize
	fmt.Printf("ChunkSize [%d]: ", cfg.ChunkSize)
	if v := readLine(reader); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ChunkSize = n
		}
	}

	// FanoutCount
	fmt.Printf("FanoutCount [%d]: ", cfg.FanoutCount)
	if v := readLine(reader); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.FanoutCount = n
		}
	}

	// 是否立即启动
	fmt.Print("是否立即启动节点? [Y/n]: ")
	startNow := strings.ToLower(readLine(reader))
	if startNow == "n" {
		fmt.Println("配置完成，未启动节点，程序退出")
		return
	}

	// 启动
	fmt.Println("\n正在启动 Gossip 节点...")
	node := NodeManage.NewChunkNode(cfg)
	if err := node.Start(); err != nil {
		fmt.Printf("启动失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("节点启动成功")
	fmt.Printf("Node Hash: %s\n\n", node.GetNodeHash())

	// 进入 CLI
	c := cli.NewCLI(node)
	c.Run()
}

func readLine(r *bufio.Reader) string {
	text, _ := r.ReadString('\n')
	return strings.TrimSpace(text)
}
