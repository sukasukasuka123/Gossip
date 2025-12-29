package SlidingWindow

import (
	"context"
)

// ResourceManage 循环将缓存推进滑动窗口
func (s *SlidingWindowManager[T]) ResourceManage(ctx context.Context, handler func(T)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.mu.Lock()
			if len(s.cacheList) == 0 {
				s.mu.Unlock()
				continue
			}

			// 检查窗口是否满
			if len(s.windowChan) >= s.maxWindow {
				s.mu.Unlock()
				continue
			}

			// 从缓存中取第一个
			key := s.cacheList[0]
			resource := s.cacheMap[key]

			// 推入窗口
			s.windowChan <- resource

			// 从缓存移除
			s.cacheList = s.cacheList[1:]
			delete(s.cacheMap, key)
			s.mu.Unlock()
		}

		// 处理窗口资源
		select {
		case <-ctx.Done():
			return
		case res := <-s.windowChan:
			handler(res)
		}
	}
}
