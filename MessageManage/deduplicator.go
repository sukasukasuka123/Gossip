// MessageManage/deduplicator.go
package MessageManage

import (
	"sync"
	"time"
)

type MessageDeduplicator struct {
	mu   sync.RWMutex
	seen map[string]time.Time // payloadHash -> 首次见到的时间
	ttl  time.Duration        // 记录保留时长
}

// NewMessageDeduplicator 创建消息去重器
func NewMessageDeduplicator(ttl time.Duration) *MessageDeduplicator {
	d := &MessageDeduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
	// 启动清理协程
	go d.cleanup()
	return d
}

// IsDuplicate 检查消息是否已见过（只读检查）
func (d *MessageDeduplicator) IsDuplicate(payloadHash string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.seen[payloadHash]
	return exists
}

// MarkAsSeen 标记消息为已见
func (d *MessageDeduplicator) MarkAsSeen(payloadHash string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.seen[payloadHash] = time.Now()
}

// cleanup 定期清理过期记录
func (d *MessageDeduplicator) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		d.mu.Lock()
		now := time.Now()
		for hash, seenTime := range d.seen {
			if now.Sub(seenTime) > d.ttl {
				delete(d.seen, hash)
			}
		}
		d.mu.Unlock()
	}
}

// Count 获取当前去重记录数量
func (d *MessageDeduplicator) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.seen)
}

// Clear 清空所有记录
func (d *MessageDeduplicator) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen = make(map[string]time.Time)
}
