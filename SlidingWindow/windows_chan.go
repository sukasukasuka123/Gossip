package SlidingWindow

import (
	"context"
	"sync/atomic"
)

// Release 由 ACK 调用，显式释放窗口
func (s *SlidingWindowManager[T]) Release() {
	select {
	case <-s.windowChan:
	default:
	}
}

// ResourceManage：推进缓存，占窗口，然后交给 handler
func (s *SlidingWindowManager[T]) ResourceManage(ctx context.Context, handler func(key string, resource T)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 检查是否有可用窗口
		if len(s.windowChan) >= s.maxWindow {
			continue
		}

		head := atomic.LoadInt32(&s.head)
		idx := head % s.maxCap
		ptr := &s.cacheList[idx]
		keyPtr := ptr.Swap(nil)
		if keyPtr == nil {
			continue
		}

		atomic.AddInt32(&s.head, 1)
		s.windowChan <- struct{}{}

		value, ok := s.cacheMap.LoadAndDelete(*keyPtr)
		if ok {
			handler(*keyPtr, value.(T))
		} else {
			// 防御：map里不存在
			<-s.windowChan
		}
	}
}
