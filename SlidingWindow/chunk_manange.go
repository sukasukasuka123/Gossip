package SlidingWindow

import (
	"sync"
)

// 批量大小可调
const DefaultBatchSize = 8

type SlidingWindowManager[T comparable] struct {
	// Pending 队列
	cacheMap  map[string]T
	cacheList []string

	// InFlight（真正的窗口）
	windowChan chan struct{}
	maxWindow  int

	mu sync.Mutex
}

func NewSlidingWindowManager[T comparable](windowSize int) *SlidingWindowManager[T] {
	s := &SlidingWindowManager[T]{
		cacheMap:   make(map[string]T),
		cacheList:  make([]string, 0),
		windowChan: make(chan struct{}, windowSize),
		maxWindow:  windowSize,
	}

	return s
}

func (s *SlidingWindowManager[T]) PushResource(key string, resource T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.cacheMap[key]; ok {
		return
	}
	s.cacheMap[key] = resource
	s.cacheList = append(s.cacheList, key)
}

func (s *SlidingWindowManager[T]) ResetOnReconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清空 inflight（旧连接的 ACK 永远不会再来）
	for len(s.windowChan) > 0 {
		<-s.windowChan
	}

	// cacheList 是否清空取决于你要不要重发
}
