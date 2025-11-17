package Storage

import (
	"log"
	"sync"
	// 假设 atomicState 和 StateMap 的定义在此
)

// LocalStorage 实现 (推荐)
type LocalStorage struct {
	states sync.Map // hash -> *atomicState
}

func NewLocalStorage() *LocalStorage {
	return &LocalStorage{
		// 'seen' map 已被移除
	}
}

// InitState 是现在唯一的入口点
func (s *LocalStorage) InitState(hash string, neighbors []string) (bool, error) {
	// 1. 检查是否已存在
	_, loaded := s.states.Load(hash)
	if loaded {
		return false, nil // 不是新消息
	}

	// 2. 准备新状态
	initial := make(StateMap)
	for _, n := range neighbors {
		initial[n] = false
	}
	state := &atomicState{}
	state.Store(initial)

	// 3. 尝试原子地存储
	// LoadOrStore 确保只有一个协程能成功存入
	_, loaded = s.states.LoadOrStore(hash, state)

	// 4. 返回结果
	// !loaded 为 true 意味着我们是第一个成功存入的
	return !loaded, nil
}

// 获取消息下指定节点的状态
func (s *LocalStorage) GetState(hash, nodeHash string) bool {
	val, ok := s.states.Load(hash)
	if !ok {
		return false
	}
	state := val.(*atomicState).Load()
	return state[nodeHash]
}

// 更新消息下节点状态
func (s *LocalStorage) UpdateState(hash string, from string) error {
	val, ok := s.states.Load(hash)
	if !ok {
		return nil
	}
	state := val.(*atomicState)

	for {
		oldMap := state.ptr.Load()
		if oldMap == nil {
			return nil
		}

		if (*oldMap)[from] {
			return nil
		}

		newMap := make(StateMap, len(*oldMap))
		for k, v := range *oldMap {
			newMap[k] = v
		}
		newMap[from] = true
		log.Printf("UpdateState: %v -> %v", oldMap, newMap)

		if state.ptr.CompareAndSwap(oldMap, &newMap) {
			return nil
		}
	}
}

// 标记已发送内容
func (s *LocalStorage) MarkSent(hash, nodeHash string) {
	val, ok := s.states.Load(hash)
	if !ok {
		return
	}
	state := val.(*atomicState)

	for {
		oldMap := state.ptr.Load()
		if oldMap == nil {
			return
		}
		if (*oldMap)[nodeHash] {
			return
		}

		newMap := make(StateMap, len(*oldMap))
		for k, v := range *oldMap {
			newMap[k] = v
		}
		newMap[nodeHash] = true

		if state.ptr.CompareAndSwap(oldMap, &newMap) {
			return
		}
	}
}

func (s *LocalStorage) GetStates(hash string) map[string]bool {
	val, ok := s.states.Load(hash)
	if !ok {
		return nil // 返回 nil map
	}
	return val.(*atomicState).Load()
}

// DeleteState (修复版)
func (s *LocalStorage) DeleteState(hash string) {
	s.states.Delete(hash) // 只需删除 states
}
