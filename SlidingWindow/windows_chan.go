package SlidingWindow

import (
	"context"
)

// Release 由 ACK 调用，显式释放窗口
func (s *SlidingWindowManager[T]) Release() {
	select {
	case <-s.windowChan:
	default:
		// 防御：避免误释放
	}
}

// ResourceManage：推进缓存，占窗口，然后交给 handler
func (s *SlidingWindowManager[T]) ResourceManage(
	ctx context.Context,
	handler func(key string, resource T),
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.mu.Lock()
		if len(s.cacheList) == 0 || len(s.windowChan) >= s.maxWindow {
			s.mu.Unlock()
			continue
		}

		key := s.cacheList[0]
		resource := s.cacheMap[key]

		// 占一个窗口
		s.windowChan <- struct{}{}

		// 从缓存移除
		s.cacheList = s.cacheList[1:]
		delete(s.cacheMap, key)
		s.mu.Unlock()

		// 交给上层处理（发包）
		handler(key, resource)
	}
}
