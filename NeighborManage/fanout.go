// NeighborManage/fanout.go
package NeighborManage

import (
	"sync"
	"time"
)

// FanoutStrategy 定义 fanout 选择策略
type FanoutStrategy int

const (
	FanoutByLatency    FanoutStrategy = iota // 按延迟选择
	FanoutByRandom                           // 随机选择
	FanoutByRoundRobin                       // 轮询选择
)

// MessageFanoutState 消息的 fanout 状态
type MessageFanoutState struct {
	MessageHash   string
	NeighborsSent map[string]bool // neighborHash -> 是否已发送
	CreatedAt     time.Time
	CompletedAt   time.Time
	IsCompleted   bool
}

// FanoutManager 负责管理消息的 fanout 策略和状态
type FanoutManager struct {
	mu              sync.RWMutex
	states          map[string]*MessageFanoutState // messageHash -> state
	strategy        FanoutStrategy
	fanoutCount     int           // 每条消息转发给多少个邻居
	stateRetention  time.Duration // 状态保留时长
	roundRobinIndex int           // 轮询索引
}

// NewFanoutManager 创建 fanout 管理器
func NewFanoutManager(strategy FanoutStrategy, fanoutCount int, stateRetention time.Duration) *FanoutManager {
	fm := &FanoutManager{
		states:         make(map[string]*MessageFanoutState),
		strategy:       strategy,
		fanoutCount:    fanoutCount,
		stateRetention: stateRetention,
	}

	// 启动状态清理协程
	go fm.cleanupStates()

	return fm
}

// SelectNeighborsForFanout 根据策略选择邻居进行 fanout
// excludeHash: 排除的邻居哈希（通常是消息发送者）
func (fm *FanoutManager) SelectNeighborsForFanout(
	messageHash string,
	allNeighbors []*NeighborNode,
	excludeHash string,
) []*NeighborNode {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 初始化或获取消息状态
	state, exists := fm.states[messageHash]
	if !exists {
		state = &MessageFanoutState{
			MessageHash:   messageHash,
			NeighborsSent: make(map[string]bool),
			CreatedAt:     time.Now(),
			IsCompleted:   false,
		}
		fm.states[messageHash] = state
	}

	// 过滤可用邻居：排除已发送的、排除指定的、只保留未发送的
	var candidates []*NeighborNode
	for _, neighbor := range allNeighbors {
		if neighbor.NodeHash == excludeHash {
			continue // 排除消息来源
		}
		if state.NeighborsSent[neighbor.NodeHash] {
			continue // 已经发送过
		}
		candidates = append(candidates, neighbor)
	}

	// 如果没有可用候选者，返回空
	if len(candidates) == 0 {
		return nil
	}

	// 根据策略选择邻居
	var selected []*NeighborNode
	switch fm.strategy {
	case FanoutByLatency:
		selected = fm.selectByLatency(candidates, fm.fanoutCount)
	case FanoutByRandom:
		selected = fm.selectByRandom(candidates, fm.fanoutCount)
	case FanoutByRoundRobin:
		selected = fm.selectByRoundRobin(candidates, fm.fanoutCount)
	default:
		selected = fm.selectByLatency(candidates, fm.fanoutCount)
	}

	// 标记已选择的邻居
	for _, neighbor := range selected {
		state.NeighborsSent[neighbor.NodeHash] = true
	}

	return selected
}

// selectByLatency 按延迟选择（已排序，取前 N 个）
func (fm *FanoutManager) selectByLatency(candidates []*NeighborNode, count int) []*NeighborNode {
	// 候选列表已经在外部按 Ping 排序
	if count > len(candidates) {
		count = len(candidates)
	}
	return candidates[:count]
}

// selectByRandom 随机选择
func (fm *FanoutManager) selectByRandom(candidates []*NeighborNode, count int) []*NeighborNode {
	if count > len(candidates) {
		count = len(candidates)
	}

	// 使用 Fisher-Yates shuffle
	shuffled := make([]*NeighborNode, len(candidates))
	copy(shuffled, candidates)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := time.Now().UnixNano() % int64(i+1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:count]
}

// selectByRoundRobin 轮询选择
func (fm *FanoutManager) selectByRoundRobin(candidates []*NeighborNode, count int) []*NeighborNode {
	if count > len(candidates) {
		count = len(candidates)
	}

	selected := make([]*NeighborNode, 0, count)
	total := len(candidates)

	for i := 0; i < count; i++ {
		idx := (fm.roundRobinIndex + i) % total
		selected = append(selected, candidates[idx])
	}

	fm.roundRobinIndex = (fm.roundRobinIndex + count) % total
	return selected
}

// MarkMessageSent 标记消息已发送给某个邻居
func (fm *FanoutManager) MarkMessageSent(messageHash, neighborHash string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if state, exists := fm.states[messageHash]; exists {
		state.NeighborsSent[neighborHash] = true
	}
}

// MarkMessageComplete 标记消息 fanout 完成
func (fm *FanoutManager) MarkMessageComplete(messageHash string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if state, exists := fm.states[messageHash]; exists {
		state.IsCompleted = true
		state.CompletedAt = time.Now()
	}
}

// GetMessageState 获取消息的 fanout 状态
func (fm *FanoutManager) GetMessageState(messageHash string) (*MessageFanoutState, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	state, exists := fm.states[messageHash]
	if !exists {
		return nil, false
	}

	// 返回副本
	stateCopy := &MessageFanoutState{
		MessageHash:   state.MessageHash,
		NeighborsSent: make(map[string]bool),
		CreatedAt:     state.CreatedAt,
		CompletedAt:   state.CompletedAt,
		IsCompleted:   state.IsCompleted,
	}
	for k, v := range state.NeighborsSent {
		stateCopy.NeighborsSent[k] = v
	}

	return stateCopy, true
}

// HasSentToNeighbor 检查是否已发送给某个邻居
func (fm *FanoutManager) HasSentToNeighbor(messageHash, neighborHash string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if state, exists := fm.states[messageHash]; exists {
		return state.NeighborsSent[neighborHash]
	}
	return false
}

// cleanupStates 定期清理过期状态
func (fm *FanoutManager) cleanupStates() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		fm.mu.Lock()
		now := time.Now()
		for hash, state := range fm.states {
			// 已完成的消息，保留一段时间后删除
			if state.IsCompleted && now.Sub(state.CompletedAt) > fm.stateRetention {
				delete(fm.states, hash)
			}
			// 未完成但创建时间过长的消息，也删除（防止内存泄漏）
			if !state.IsCompleted && now.Sub(state.CreatedAt) > fm.stateRetention*2 {
				delete(fm.states, hash)
			}
		}
		fm.mu.Unlock()
	}
}

// GetStats 获取 fanout 统计信息
func (fm *FanoutManager) GetStats() map[string]interface{} {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	totalStates := len(fm.states)
	completedCount := 0

	for _, state := range fm.states {
		if state.IsCompleted {
			completedCount++
		}
	}

	return map[string]interface{}{
		"total_messages":     totalStates,
		"completed_messages": completedCount,
		"pending_messages":   totalStates - completedCount,
		"fanout_count":       fm.fanoutCount,
		"strategy":           fm.strategy,
	}
}
