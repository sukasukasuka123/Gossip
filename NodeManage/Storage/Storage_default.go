package Storage

import (
	"log"
	"sync"
)

// LocalStorage 实现
type LocalStorage struct {
	seen   sync.Map // hash -> struct{}
	states sync.Map // hash -> *atomicState
}

func NewLocalStorage() *LocalStorage {
	return &LocalStorage{}
}

func (s *LocalStorage) Has(hash string) bool {
	_, loaded := s.seen.Load(hash)
	return loaded
}

func (s *LocalStorage) Mark(hash string) {
	s.seen.Store(hash, struct{}{})
}

func (s *LocalStorage) InitState(hash string, neighbors []string) error {
	_, loaded := s.states.Load(hash)
	if loaded {
		return nil
	}

	initial := make(StateMap)
	for _, n := range neighbors {
		initial[n] = false
	}

	state := &atomicState{}
	state.Store(initial)
	s.states.Store(hash, state)
	return nil
}

func (s *LocalStorage) UpdateState(hash string, from string) error {
	val, ok := s.states.Load(hash)
	if !ok {
		return nil
	}
	state := val.(*atomicState)

	for {
		oldMap := state.ptr.Load()
		if oldMap == nil {
			return nil
		}

		if (*oldMap)[from] {
			return nil
		}

		newMap := make(StateMap, len(*oldMap))
		for k, v := range *oldMap {
			newMap[k] = v
		}
		newMap[from] = true
		log.Printf("UpdateState: %v -> %v", oldMap, newMap)

		if state.ptr.CompareAndSwap(oldMap, &newMap) {
			return nil
		}
	}
}

func (s *LocalStorage) GetStates(hash string) (map[string]bool, error) {
	val, ok := s.states.Load(hash)
	if !ok {
		return nil, nil
	}
	return val.(*atomicState).Load(), nil
}

func (s *LocalStorage) GetState(hash, nodeHash string) bool {
	val, ok := s.states.Load(hash)
	if !ok {
		return false
	}
	state := val.(*atomicState).Load()
	return state[nodeHash]
}

func (s *LocalStorage) MarkSent(hash, nodeHash string) {
	val, ok := s.states.Load(hash)
	if !ok {
		return
	}
	state := val.(*atomicState)

	for {
		oldMap := state.ptr.Load()
		if oldMap == nil {
			return
		}
		if (*oldMap)[nodeHash] {
			return
		}

		newMap := make(StateMap, len(*oldMap))
		for k, v := range *oldMap {
			newMap[k] = v
		}
		newMap[nodeHash] = true

		if state.ptr.CompareAndSwap(oldMap, &newMap) {
			return
		}
	}
}

func (s *LocalStorage) DeleteState(hash string) {
	s.seen.Delete(hash)
}
