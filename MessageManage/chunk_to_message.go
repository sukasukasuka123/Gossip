package MessageManage

// 这是消息重组模块，负责将分块消息重组为完整消息

import "sync"

type ReassembleState struct {
	total    int32
	received map[int32][]byte
}

type Reassembler struct {
	mu    sync.Mutex
	state map[string]*ReassembleState // payloadHash -> state
}

func NewReassembler() *Reassembler {
	return &Reassembler{
		state: make(map[string]*ReassembleState),
	}
}

func (r *Reassembler) AddChunk(
	payloadHash string,
	index int32,
	total int32,
	data []byte,
) (complete bool, payload []byte) {

	r.mu.Lock()
	defer r.mu.Unlock()

	st, ok := r.state[payloadHash]
	if !ok {
		st = &ReassembleState{
			total:    total,
			received: make(map[int32][]byte),
		}
		r.state[payloadHash] = st
	}

	if _, exists := st.received[index]; exists {
		return false, nil
	}

	st.received[index] = data

	if int32(len(st.received)) != st.total {
		return false, nil
	}

	// 重组
	for i := int32(0); i < st.total; i++ {
		payload = append(payload, st.received[i]...)
	}

	delete(r.state, payloadHash)
	return true, payload
}
