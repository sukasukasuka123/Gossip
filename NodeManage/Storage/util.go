package Storage

import "sync/atomic"

// 消息状态映射
type StateMap map[string]bool

// atomicState 使用 atomic.Pointer 保证 CAS 安全
type atomicState struct {
	ptr atomic.Pointer[StateMap]
}

func (a *atomicState) Load() StateMap {
	p := a.ptr.Load()
	if p == nil {
		return nil
	}
	cp := make(StateMap, len(*p))
	for k, v := range *p {
		cp[k] = v
	}
	return cp
}

func (a *atomicState) CompareAndSwap(old, new StateMap) bool {
	return a.ptr.CompareAndSwap(&old, &new)
}

func (a *atomicState) Store(m StateMap) {
	a.ptr.Store(&m)
}
