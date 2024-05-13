package future

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

var once sync.Once
var Manager *FutureManager

// FutureManager Future管理器
type FutureManager struct {
	mutex   sync.Mutex
	futures map[string]*Future
}

// NewFuture 创建Future
func (m *FutureManager) NewFuture() *Future {
	token := make([]byte, 8)
	_, err := rand.Read(token)
	if err != nil {
		return nil
	}
	future := newFuture(hex.EncodeToString(token))
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.futures[future.id] = future
	return future
}

// SetResult 设置结果
func (m *FutureManager) SetResult(id, txid string, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	future, ok := m.futures[id]
	if !ok {
		return
	}
	future.SetResult(txid, err)
	delete(m.futures, id)
}

// NewFutureManagerOnce 创建Future管理器
func NewFutureManagerOnce() {
	once.Do(func() {
		Manager = &FutureManager{futures: make(map[string]*Future)}
	})
}
