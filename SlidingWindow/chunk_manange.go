package SlidingWindow

import (
	"sync"
)

// 批量大小可调
const DefaultBatchSize = 8

type SlidingWindowManager[T comparable] struct {
	cacheMap  map[string]T
	cacheList []string
	mu        sync.Mutex

	windowChan chan struct{}
	maxWindow  int
}

func NewSlidingWindowManager[T comparable](windowSize int) *SlidingWindowManager[T] {
	return &SlidingWindowManager[T]{
		cacheMap:   make(map[string]T),
		cacheList:  make([]string, 0),
		windowChan: make(chan struct{}, windowSize),
		maxWindow:  windowSize,
	}
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
