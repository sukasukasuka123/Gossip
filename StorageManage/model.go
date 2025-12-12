package StorageManage

import (
	"errors"
	"time"

	"github.com/sukasukasuka123/Gossip/Storage"
)

// StorageRequest 表示所有对存储的操作，统一入口
type StorageRequest struct {
	Op      StorageOp
	Payload Payload
	Reply   chan interface{} // nil 表示不需要返回
}

type StorageManage struct {
	storage IStorage
	reqChan chan StorageRequest // 统一入口
}

func NewStorageManage(custom IStorage) *StorageManage {
	sm := &StorageManage{
		reqChan: make(chan StorageRequest, 256), // 放宽缓冲区
	}

	if custom != nil {
		sm.storage = custom
	} else {
		sm.storage = Storage.NewLocalStorage(50, 2*time.Minute)
	}

	go sm.loop()
	return sm
}

func (sm *StorageManage) loop() {
	for req := range sm.reqChan {
		sm.dispatch(req)
	}
}

func (sm *StorageManage) dispatch(req StorageRequest) {
	var result interface{}
	var err error

	switch req.Op {

	case OpInitState:
		err = sm.handleInit(req.Payload)

	case OpUpdateState:
		err = sm.handleUpdate(req.Payload)

	case OpMarkSent:
		err = sm.handleMarkSent(req.Payload)

	case OpDeleteState:
		err = sm.handleDelete(req.Payload)

	case OpGetState:
		result, err = sm.handleGetState(req.Payload)

	case OpGetAllStates:
		result, err = sm.handleGetAllStates(req.Payload)

	default:
		err = errors.New("unknown op")
	}

	if req.Reply != nil {
		if err != nil {
			req.Reply <- err
			return
		}
		req.Reply <- result
	}
}

// ===== 下面是每个操作的具体函数 =====

func (sm *StorageManage) handleInit(p Payload) error {
	if p.Neighbors == nil {
		return errors.New("InitState requires Neighbors")
	}
	_, err := sm.storage.InitState(p.Hash, p.Neighbors)
	return err
}

func (sm *StorageManage) handleUpdate(p Payload) error {
	if p.NodeHash == "" {
		return errors.New("UpdateState requires NodeHash")
	}
	return sm.storage.UpdateState(p.Hash, p.NodeHash)
}

func (sm *StorageManage) handleMarkSent(p Payload) error {
	if p.NodeHash == "" {
		return errors.New("MarkSent requires NodeHash")
	}
	sm.storage.MarkSent(p.Hash, p.NodeHash)
	return nil
}

func (sm *StorageManage) handleDelete(p Payload) error {
	sm.storage.DeleteState(p.Hash)
	return nil
}

func (sm *StorageManage) handleGetState(p Payload) (interface{}, error) {
	if p.NodeHash == "" {
		return nil, errors.New("GetState requires NodeHash")
	}
	return sm.storage.GetState(p.Hash, p.NodeHash), nil
}

func (sm *StorageManage) handleGetAllStates(p Payload) (interface{}, error) {
	return sm.storage.GetStates(p.Hash), nil
}

// ===== 公共接口（上层调用这些）=====

// 异步操作（不需要返回）
func (sm *StorageManage) Schedule(op StorageOp, payload Payload) {
	sm.reqChan <- StorageRequest{
		Op:      op,
		Payload: payload,
		Reply:   nil,
	}
}

// 同步操作（需要返回）
func (sm *StorageManage) ScheduleSync(op StorageOp, payload Payload) (interface{}, error) {
	reply := make(chan interface{}, 1)
	sm.reqChan <- StorageRequest{
		Op:      op,
		Payload: payload,
		Reply:   reply,
	}
	res := <-reply
	if err, ok := res.(error); ok {
		return nil, err
	}
	return res, nil
}
