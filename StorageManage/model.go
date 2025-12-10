package StorageManage

import (
	"errors"
	"time"

	"github.com/sukasukasuka123/Gossip/Storage"
)

// StorageRequest 写操作请求
type StorageRequest struct {
	Op      StorageOp
	Payload Payload
	Reply   chan interface{} // 用于读操作返回值
}

// StorageManage 是统一的调度入口
type StorageManage struct {
	storage   IStorage
	writeChan chan StorageRequest
	readChan  chan StorageRequest
}

// NewStorageManage 初始化
func NewStorageManage(custom IStorage) *StorageManage {
	sm := &StorageManage{
		writeChan: make(chan StorageRequest, 128),
		readChan:  make(chan StorageRequest, 128),
	}

	if custom != nil {
		sm.storage = custom
	} else {
		sm.storage = Storage.NewLocalStorage(50, 2*time.Minute)
	}

	// 启动调度协程
	go sm.loop()

	return sm
}

// loop 调度协程
func (sm *StorageManage) loop() {
	for {
		select {
		case req := <-sm.writeChan:
			sm.handleWrite(req)
		case req := <-sm.readChan:
			sm.handleRead(req)
		}
	}
}

// handleWrite 处理写操作
func (sm *StorageManage) handleWrite(req StorageRequest) {
	switch req.Op {
	case OpInitState:
		if req.Payload.Neighbors != nil {
			sm.storage.InitState(req.Payload.Hash, req.Payload.Neighbors)
		}
	case OpUpdateState:
		if req.Payload.NodeHash != "" {
			sm.storage.UpdateState(req.Payload.Hash, req.Payload.NodeHash)
		}
	case OpMarkSent:
		if req.Payload.NodeHash != "" {
			sm.storage.MarkSent(req.Payload.Hash, req.Payload.NodeHash)
		}
	case OpDeleteState:
		sm.storage.DeleteState(req.Payload.Hash)
	}
	// 写操作无需返回值
	if req.Reply != nil {
		req.Reply <- nil
	}
}

// handleRead 处理读操作
func (sm *StorageManage) handleRead(req StorageRequest) {
	var result interface{}
	var err error

	switch req.Op {
	case OpGetState:
		if req.Payload.NodeHash != "" {
			result = sm.storage.GetState(req.Payload.Hash, req.Payload.NodeHash)
		} else {
			err = errors.New("OpGetState requires NodeHash")
		}
	case OpGetAllStates:
		result = sm.storage.GetStates(req.Payload.Hash)
	default:
		err = errors.New("unknown read operation")
	}

	if req.Reply != nil {
		if err != nil {
			req.Reply <- err
		} else {
			req.Reply <- result
		}
	}
}

// 公共接口：调度写操作
func (sm *StorageManage) ScheduleWrite(op StorageOp, payload Payload) {
	sm.writeChan <- StorageRequest{
		Op:      op,
		Payload: payload,
		Reply:   nil,
	}
}

// 公共接口：调度读操作并等待返回
func (sm *StorageManage) ScheduleRead(op StorageOp, payload Payload) (interface{}, error) {
	reply := make(chan interface{}, 1)
	sm.readChan <- StorageRequest{
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
