package context

import (
	"sync"

	"github.com/zhangpanyi/basebot/history"
	"github.com/zhangpanyi/basebot/logger"
)

var once sync.Once
var manager *history.Manager

// CreateManagerOnce 创建记录管理器
func CreateManagerOnce(bucketNum uint32) {
	once.Do(func() {
		var err error
		manager, err = history.NewManager(bucketNum)
		if err != nil {
			logger.Panicf("Failed to create manager for lucky money, %v", err)
		}
	})
}

// DelRecord 删除记录
func DelRecord(userID uint32) {
	manager.Del(userID)
}

// GetRecord 获取记录
func GetRecord(userID uint32) (*history.History, error) {
	return manager.Get(userID)
}
