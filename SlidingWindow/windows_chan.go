package SlidingWindow

import (
	"context"
	"fmt"
	"time"
)

// 批量释放窗口
func (s *SlidingWindowManager[T]) ReleaseBatch(n int) {
	for i := 0; i < n; i++ {
		select {
		case <-s.windowChan:
			// 成功释放一个 inflight
			fmt.Printf("[SlidingWindow] ReleaseBatch: released 1 slot, current inflight=%d\n", len(s.windowChan))
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

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("[SlidingWindow] ResourceManageBatch: context done, exiting")
			return
		case <-ticker.C:
		}

		s.mu.Lock()

		cacheLen := len(s.cacheList)
		inflight := len(s.windowChan)

		// 窗口满 or 没数据 → 打印状态并跳过
		if cacheLen == 0 || inflight >= s.maxWindow {
			s.mu.Unlock()
			continue
		}

		// 计算本次最多能发多少
		n := batchSize
		if n > cacheLen {
			n = cacheLen
		}
		if n > (s.maxWindow - inflight) {
			n = s.maxWindow - inflight
		}

		batch := make(map[string]T, n)

		for i := 0; i < n; i++ {
			key := s.cacheList[0]
			resource := s.cacheMap[key]

			// pending → inflight
			s.cacheList = s.cacheList[1:]
			delete(s.cacheMap, key)

			// 占用一个窗口槽位（表示 inflight）
			s.windowChan <- struct{}{}

			batch[key] = resource
		}

		fmt.Printf("[SlidingWindow] dispatching batch: size=%d, inflight(before send)=%d, cache(after dispatch)=%d\n",
			len(batch), inflight, len(s.cacheList))

		s.mu.Unlock()

		// 真正发送（是否成功由 ACK 决定）
		handler(batch)

		// 发送完成后可以打印每个 key
		for key := range batch {
			fmt.Printf("[SlidingWindow] sent resource key=%s\n", key)
		}
	}
}
