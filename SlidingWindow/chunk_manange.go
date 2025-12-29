package SlidingWindow

import (
	"sync"
)

// 缓存 + 滑动窗口管理
type SlidingWindowManager[T comparable] struct {
	cacheMap  map[string]T // key -> resource, 可用 hash 或 ID
	cacheList []string     // 维持顺序
	mu        sync.Mutex

	windowChan chan T // 滑动窗口通道
	maxWindow  int
}

// 创建 SlidingWindowManager
func NewSlidingWindowManager[T comparable](windowSize int) *SlidingWindowManager[T] {
	return &SlidingWindowManager[T]{
		cacheMap:   make(map[string]T),
		cacheList:  make([]string, 0),
		windowChan: make(chan T, windowSize),
		maxWindow:  windowSize,
	}
}

// PushResource 入缓存
func (s *SlidingWindowManager[T]) PushResource(key string, resource T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 去重
	if _, exists := s.cacheMap[key]; exists {
		return
	}

	s.cacheMap[key] = resource
	s.cacheList = append(s.cacheList, key)
}
