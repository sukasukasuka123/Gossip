package Storage

import (
	"sync"
	"sync/atomic"
	"time"
)

// LocalStorage 实现
type LocalStorage struct {
	states    sync.Map
	seenCache sync.Map

	shortTTL   atomic.Int64 // 当前 TTL
	shortLimit atomic.Int64 // max TTL
	ttlChan    chan TTLEvent

	longTTL time.Duration
}

func NewLocalStorage(shortTTLSec int64, longTTL time.Duration) *LocalStorage {
	s := &LocalStorage{
		ttlChan: make(chan TTLEvent, 32),
		longTTL: longTTL,
	}
	s.shortTTL.Store(shortTTLSec)
	s.shortLimit.Store(shortTTLSec * 2) // 初始 limit = 2x，可调

	go s.cleanupSeenCacheLoop()
	go s.ttlLoop() // 核心反应堆

	return s
}

// 初始化消息状态
func (s *LocalStorage) InitState(hash string, neighbors []string) (bool, error) {
	if _, seen := s.seenCache.Load(hash); seen {
		return false, nil
	}

	s.seenCache.Store(hash, time.Now())

	initial := make(StateMap)
	for _, n := range neighbors {
		initial[n] = false
	}
	state := &atomicState{}
	state.Store(initial)
	s.states.Store(hash, state)

	// 使用 atomic shortTTL 设置 TTL
	ttl := time.Duration(s.shortTTL.Load()) * time.Second
	time.AfterFunc(ttl, func() {
		s.states.Delete(hash)
	})

	return true, nil
}

// 获取消息下指定节点状态
func (s *LocalStorage) GetState(hash, nodeHash string) bool {
	val, ok := s.states.Load(hash)
	if !ok {
		return false
	}
	state := val.(*atomicState).Load()
	return state[nodeHash]
}

// 更新消息状态
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

// 获取消息所有节点状态
func (s *LocalStorage) GetStates(hash string) map[string]bool {
	val, ok := s.states.Load(hash)
	if !ok {
		return nil
	}
	return val.(*atomicState).Load()
}

// 删除消息状态
func (s *LocalStorage) DeleteState(hash string) {
	s.states.Delete(hash)
}

// 延迟清理 seenCache
func (s *LocalStorage) cleanupSeenCacheLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		now := time.Now()
		s.seenCache.Range(func(key, value interface{}) bool {
			t := value.(time.Time)
			if now.Sub(t) > s.longTTL {
				s.seenCache.Delete(key)
			}
			return true
		})
	}
}

// 原子更新 shortLimit
func (s *LocalStorage) UpdateShortLimit(nodeNum int) int64 {
	cur := s.shortLimit.Load()

	// 判断条件：新邻居数量 * 2 > 当前 Limit
	if int64(nodeNum)*2 <= cur {
		return cur // 不增长
	}

	// 做指数增长
	newLimit := cur * 2

	// 不超过 64s
	if newLimit > 64 {
		newLimit = 64
	}

	s.shortLimit.Store(newLimit)
	return newLimit
}

// // 主动清空 shortTTL 下的所有状态
// func (s *LocalStorage) ClearShortTTLStates() {
// 	s.states.Range(func(key, value interface{}) bool {
// 		s.states.Delete(key)
// 		return true
// 	})
// }
