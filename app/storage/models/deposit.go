package models

import (
	"errors"

	"github.com/boltdb/bolt"
	"luckybot/app/storage"
)

// DepositModel 充值模型
type DepositModel struct {
}

// Exist 记录是否存在
func (model *DepositModel) Exist(txid string) bool {
	ret := false
	storage.DB.View(func(tx *bolt.Tx) error {
		ret = model.exist(tx, txid)
		return nil
	})
	return ret
}

// Add 添加充值记录
func (model *DepositModel) Add(txid string, data []byte) error {
	return storage.DB.Update(func(tx *bolt.Tx) error {
		if model.exist(tx, txid) {
			return errors.New("repeat deposit")
		}
		bucket, err := storage.EnsureBucketExists(tx, "deposits")
		if err != nil {
			return nil
		}
		return bucket.Put([]byte(txid), data)
	})
}

// 查询TxID是否存在
func (model *DepositModel) exist(tx *bolt.Tx, txid string) bool {
	bucket, err := storage.GetBucketIfExists(tx, "deposits")
	if err != nil {
		if err == storage.ErrNoBucket {
			return false
		}
		return true
	}
	if bucket.Get([]byte(txid)) == nil {
		return false
	}
	return true
}
