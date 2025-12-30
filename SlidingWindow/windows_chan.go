package SlidingWindow

import (
	"context"
	"time"
)

// 批量释放窗口
func (s *SlidingWindowManager[T]) ReleaseBatch(n int) {
	for i := 0; i < n; i++ {
		select {
		case <-s.windowChan:
		default:
			return
		}
	}
}

// ResourceManageBatch 批量处理资源
func (s *SlidingWindowManager[T]) ResourceManageBatch(
	ctx context.Context,
	handler func(batch map[string]T),
	batchSize int,
) {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.mu.Lock()
		if len(s.cacheList) == 0 || len(s.windowChan) >= s.maxWindow {
			s.mu.Unlock()
			time.Sleep(time.Millisecond) // 避免 CPU 空转
			continue
		}

		// 计算批量大小
		n := batchSize
		if n > len(s.cacheList) {
			n = len(s.cacheList)
		}
		if n > (s.maxWindow - len(s.windowChan)) {
			n = s.maxWindow - len(s.windowChan)
		}

		batch := make(map[string]T, n)
		for i := 0; i < n; i++ {
			key := s.cacheList[0]
			resource := s.cacheMap[key]

			batch[key] = resource

			s.cacheList = s.cacheList[1:]
			delete(s.cacheMap, key)

			s.windowChan <- struct{}{}
		}
		s.mu.Unlock()

		// 上层处理整个批次
		handler(batch)
	}
}
