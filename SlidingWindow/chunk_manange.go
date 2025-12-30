package SlidingWindow

import (
	"sync"
	"sync/atomic"
)

// 缓存 + 滑动窗口管理
type SlidingWindowManager[T any] struct {
	cacheMap  sync.Map                 // key -> resource
	cacheList []atomic.Pointer[string] // 环形队列
	head      int32
	tail      int32
	maxCap    int32

	windowChan chan struct{}
	maxWindow  int
}

// 创建 SlidingWindowManager
func NewSlidingWindowManager[T any](windowSize int) *SlidingWindowManager[T] {
	return &SlidingWindowManager[T]{
		cacheList:  make([]atomic.Pointer[string], windowSize*2), // 环形长度大于窗口大小
		maxCap:     int32(windowSize * 2),
		windowChan: make(chan struct{}, windowSize),
		maxWindow:  windowSize,
	}
}

// PushResource 入缓存
// PushResource 入缓存（无锁）
func (s *SlidingWindowManager[T]) PushResource(key string, resource T) {
	if _, loaded := s.cacheMap.LoadOrStore(key, resource); loaded {
		return
	}

	for {
		tail := atomic.LoadInt32(&s.tail)
		idx := tail % s.maxCap
		ptr := &s.cacheList[idx]

		if ptr.CompareAndSwap(nil, &key) {
			atomic.AddInt32(&s.tail, 1)
			return
		}
		// 队列满则自旋
	}
}
