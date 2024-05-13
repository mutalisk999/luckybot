package models

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"

	"github.com/boltdb/bolt"
	"luckybot/app/fmath"
	"luckybot/app/storage"
)

// DefaultLuckyMoneyID 默认红包ID
const DefaultLuckyMoneyID = 100000

// LuckyMoney 红包信息
type LuckyMoney struct {
	ID         uint64     `json:"id"`          // 红包ID
	SN         string     `json:"sn"`          // 唯一编号
	SenderID   int64      `json:"sender_id"`   // 发送者
	SenderName string     `json:"sender_name"` // 发送者名字
	Asset      string     `json:"asset"`       // 资产类型
	Amount     *big.Float `json:"amount"`      // 红包金额
	Received   *big.Float `json:"received"`    // 领取金额
	Number     uint32     `json:"number"`      // 红包个数
	Lucky      bool       `json:"lucky"`       // 是否随机
	Value      *big.Float `json:"value"`       // 单个价值
	Active     bool       `json:"active"`      // 是否激活
	Message    string     `json:"message"`     // 红包留言
	Timestamp  int64      `json:"timestamp"`   // 时间戳
}

// Normalization 标准化
func (luckymoney *LuckyMoney) Normalization() {
	if luckymoney.Value != nil {
		luckymoney.Value.SetPrec(fmath.Prec())
	}
	if luckymoney.Amount != nil {
		luckymoney.Amount.SetPrec(fmath.Prec())
	}
	if luckymoney.Received != nil {
		luckymoney.Received.SetPrec(fmath.Prec())
	}
}

// LuckyMoneyUser 红包用户
type LuckyMoneyUser struct {
	UserID    int64  `json:"user_id"`    // 用户ID
	FirstName string `json:"first_name"` // 用户名
}

// LuckyMoneyHistory 红包记录
type LuckyMoneyHistory struct {
	Value *big.Float      `json:"value"`          // 红包金额
	User  *LuckyMoneyUser `json:"user,omitempty"` // 用户信息
}

// Normalization 标准化
func (history *LuckyMoneyHistory) Normalization() {
	if history.Value != nil {
		history.Value.SetPrec(fmath.Prec())
	}
}

var (
	// ErrNothingLeft 领完了
	ErrNothingLeft = errors.New("nothing left")
	// ErrRepeatReceive 重复领取
	ErrRepeatReceive = errors.New("repeat receive")
	// ErrNotActivated 没有激活
	ErrNotActivated = errors.New("not activated")
	// ErrAlreadyActivated 已经激活
	ErrAlreadyActivated = errors.New("already activated")
	// ErrPermissionDenied 没有权限
	ErrPermissionDenied = errors.New("permission denied")
	// ErrLuckyMoneydExpired 红包已过期
	ErrLuckyMoneydExpired = errors.New("lucky money expired")
)

// ********************** 结构图 **********************
// {
// 	"luckymoney": {
// 		"sid": {
// 			"seq": 0,					// 红包领取序列
// 			"worst": 0,					// 手气最烂序列
// 			"best": 0,					// 手气最佳序列
// 			"base": types.LuckyMoney	// 红包基本信息
//			"users": {					// 红包已领用户
//				"user_id": ""
//			}
// 			"history": {				// 红包领取记录
// 				"seq": types.LuckyMoneyHistory
// 			}
//			"expired": true				// 红包是否过期
// 		},
//		"mapping": {					// 红包编号映射
//			<sn>: <sid>
//		},
//		"pending": {					// 用户挂起红包
//			<user_id>: array
//		},
//		"history": {					// 用户历史红包
//			<user_id>: array
//		},
//		"sequeue": 0,					// 红包ID生成序列
//		"latest_expired": 0,		    // 最新过期红包ID
// 	}
// }
// ***************************************************

// LuckyMoneyModel 红包模型
type LuckyMoneyModel struct {
}

// 生成序列号
func (model *LuckyMoneyModel) generateSN(tx *bolt.Tx, id uint64) (string, error) {
	bucket, err := storage.EnsureBucketExists(tx, "luckymoney", "mapping")
	if err != nil {
		return "", err
	}

	sn := ""
	token := make([]byte, 8)
	for {
		_, err = rand.Read(token)
		if err != nil {
			return "", err
		}
		sn = hex.EncodeToString(token)
		if bucket.Get([]byte(token)) == nil {
			sid := strconv.FormatUint(id, 10)
			return sn, bucket.Put([]byte(sn), []byte(sid))
		}
	}
}

// 添加历史红包
func (model *LuckyMoneyModel) moveToUserHistory(tx *bolt.Tx, usdeID int64, sid string) error {
	sender := strconv.FormatInt(usdeID, 10)
	pending, err := storage.GetBucketIfExists(tx, "luckymoney", "pending", sender)
	if err != nil {
		if err != storage.ErrNoBucket {
			return err
		}
		return nil
	}
	if err = pending.Delete([]byte(sid)); err != nil {
		return err
	}

	history, err := storage.EnsureBucketExists(tx, "luckymoney", "history", sender)
	if err != nil {
		if err != storage.ErrNoBucket {
			return err
		}
		return nil
	}
	if err = history.Put([]byte(sid), []byte("#")); err != nil {
		return err
	}

	return nil
}

// 创建领取记录
func (model *LuckyMoneyModel) insertHistory(tx *bolt.Tx, sid string, luckyMoneyArr []*big.Float) (int, int, error) {

	worstSeq, bestSeq := 0, 0
	minValue, maxValue := big.NewFloat(math.MaxFloat64), big.NewFloat(0)
	bucket, err := storage.EnsureBucketExists(tx, "luckymoney", sid, "history")
	if err != nil {
		return 0, 0, err
	}
	for i := range luckyMoneyArr {
		seq, err := bucket.NextSequence()
		if err != nil {
			return 0, 0, err
		}

		val := LuckyMoneyHistory{Value: luckyMoneyArr[i]}
		jsb, err := json.Marshal(&val)
		if err != nil {
			return 0, 0, err
		}

		sseq := strconv.FormatUint(seq, 10)
		err = bucket.Put([]byte(sseq), jsb)
		if err != nil {
			return 0, 0, err
		}

		if luckyMoneyArr[i].Cmp(minValue) == -1 {
			minValue = luckyMoneyArr[i]
			worstSeq = int(seq)
		}

		if luckyMoneyArr[i].Cmp(maxValue) == 1 {
			maxValue = luckyMoneyArr[i]
			bestSeq = int(seq)
		}
	}
	return worstSeq, bestSeq, nil
}

// 领取红包
func (model *LuckyMoneyModel) receiveLuckyMoney(tx *bolt.Tx, sid string, seq int, user *LuckyMoneyUser) (*big.Float, error) {

	bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid, "history")
	if err != nil {
		return nil, err
	}

	var history LuckyMoneyHistory
	key := []byte(strconv.Itoa(seq))
	jsb := bucket.Get(key)
	if err = json.Unmarshal(jsb, &history); err != nil {
		return nil, err
	}
	history.Normalization()
	history.User = user

	jsb, err = json.Marshal(&history)
	if err != nil {
		return nil, err
	}

	if err = bucket.Put(key, jsb); err != nil {
		return nil, err
	}
	return history.Value, nil
}

// NewLuckyMoney 创建新红包
func (model *LuckyMoneyModel) NewLuckyMoney(data *LuckyMoney, luckyMoneyArr []*big.Float) (*LuckyMoney, error) {
	err := storage.DB.Update(func(tx *bolt.Tx) error {
		// 生成红包ID
		rootBucket, err := storage.EnsureBucketExists(tx, "luckymoney")
		if err != nil {
			return err
		}
		if rootBucket.Sequence() < DefaultLuckyMoneyID {
			if err = rootBucket.SetSequence(DefaultLuckyMoneyID); err != nil {
				return err
			}
		}
		data.ID, err = rootBucket.NextSequence()
		if err != nil {
			return err
		}

		// 生成序列号
		sn, err := model.generateSN(tx, data.ID)
		if err != nil {
			return err
		}
		data.SN = sn

		// 序列化数据
		data.Received = big.NewFloat(0)
		data.Active = false
		jsb, err := json.Marshal(data)
		if err != nil {
			return err
		}

		// 插入基本信息
		sid := strconv.FormatUint(data.ID, 10)
		bucket, err := storage.EnsureBucketExists(tx, "luckymoney", sid)
		if err != nil {
			return err
		}
		err = bucket.Put([]byte("base"), jsb)
		if err != nil {
			return err
		}

		// 插入领取用户
		_, err = storage.EnsureBucketExists(tx, "luckymoney", sid, "users")
		if err != nil {
			return err
		}

		// 插入领取记录
		worstSeq, bestSeq, err := model.insertHistory(tx, sid, luckyMoneyArr)
		if err != nil {
			return err
		}

		// 插入已领取序列
		err = bucket.Put([]byte("seq"), []byte("0"))
		if err != nil {
			return err
		}

		// 插入手气最佳序列
		err = bucket.Put([]byte("best"), []byte(strconv.Itoa(bestSeq)))
		if err != nil {
			return err
		}

		// 插入手气最烂序列
		err = bucket.Put([]byte("worst"), []byte(strconv.Itoa(worstSeq)))
		if err != nil {
			return err
		}

		// 更新用户红包索引
		key := strconv.FormatInt(data.SenderID, 10)
		pending, err := storage.EnsureBucketExists(tx, "luckymoney", "pending", key)
		if err != nil {
			return err
		}
		return pending.Put([]byte(sid), []byte("#"))
	})

	if err != nil {
		return nil, err
	}
	return data, nil
}

// IsExpired 是否过期
func (model *LuckyMoneyModel) IsExpired(id uint64) bool {
	var expired bool
	sid := strconv.FormatUint(id, 10)
	err := storage.DB.View(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid)
		if err != nil {
			return err
		}
		expired = bucket.Get([]byte("expired")) != nil
		return nil
	})

	if err != nil {
		return false
	}
	return expired
}

// SetExpired 设置过期
func (model *LuckyMoneyModel) SetExpired(id uint64) error {
	// 获取红包信息
	luckyMoney, _, err := model.GetLuckyMoney(id)
	if err != nil {
		return nil
	}

	sid := strconv.FormatUint(id, 10)
	return storage.DB.Update(func(tx *bolt.Tx) error {
		// 添加用户历史
		if err = model.moveToUserHistory(tx, luckyMoney.SenderID, sid); err != nil {
			return err
		}

		// 标记红包过期
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid)
		if err != nil {
			return err
		}
		return bucket.Put([]byte("expired"), []byte("true"))
	})
}

// IsReceived 是否已领取
func (model *LuckyMoneyModel) IsReceived(id uint64, userID int64) (bool, error) {
	received := false
	sid := strconv.FormatUint(id, 10)
	err := storage.DB.View(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid, "users")
		if err != nil {
			return err
		}
		received = bucket.Get([]byte(strconv.FormatInt(userID, 10))) != nil
		return nil
	})
	if err != nil {
		return received, err
	}
	return received, nil
}

// GetLatestExpired 获取最新过期红包
func (model *LuckyMoneyModel) GetLatestExpired() (uint64, error) {
	var id uint64
	err := storage.DB.View(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney")
		if err != nil {
			return err
		}

		sid := bucket.Get([]byte("latest_expired"))
		if sid == nil {
			return nil
		}

		id, err = strconv.ParseUint(string(sid), 10, 64)
		if err != nil {
			return nil
		}
		return nil
	})

	if err != nil {
		return 0, err
	}
	return id, nil
}

// SetLatestExpired 设置最新过期红包
func (model *LuckyMoneyModel) SetLatestExpired(id uint64) error {
	sid := strconv.FormatUint(id, 10)
	return storage.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney")
		if err != nil {
			return err
		}
		return bucket.Put([]byte("latest_expired"), []byte(sid))
	})
}

// GetLuckyMoney 获取红包信息
func (model *LuckyMoneyModel) GetLuckyMoney(id uint64) (*LuckyMoney, uint32, error) {
	var received uint32
	var base LuckyMoney
	sid := strconv.FormatUint(id, 10)
	err := storage.DB.View(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid)
		if err != nil {
			return err
		}

		// 获取红包信息
		jsb := bucket.Get([]byte("base"))
		if err = json.Unmarshal(jsb, &base); err != nil {
			return err
		}
		base.Normalization()

		// 已领取数量
		seq := bucket.Get([]byte("seq"))
		numReceived, err := strconv.Atoi(string(seq))
		if err != nil {
			return err
		}

		// 剩余红包数量
		received = uint32(numReceived)
		return nil
	})

	if err != nil {
		return nil, 0, err
	}
	return &base, received, nil
}

// GetLuckyMoneyIDBySN 根据SN获取红包ID
func (model *LuckyMoneyModel) GetLuckyMoneyIDBySN(sn string) (uint64, error) {
	var id uint64
	err := storage.DB.View(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", "mapping")
		if err != nil {
			return err
		}

		value := bucket.Get([]byte(sn))
		if value == nil {
			return errors.New("not found")
		}

		id, err = strconv.ParseUint(string(value), 10, 64)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return 0, err
	}
	return id, nil
}

// ReceiveLuckyMoney 领取红包
func (model *LuckyMoneyModel) ReceiveLuckyMoney(id uint64, userID int64, firstName string) (*big.Float, int, error) {
	received, err := model.IsReceived(id, userID)
	if err != nil {
		return nil, 0, err
	}

	if received {
		return nil, 0, ErrRepeatReceive
	}

	count := 0
	value := big.NewFloat(0)
	sid := strconv.FormatUint(id, 10)
	err = storage.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid)
		if err != nil {
			return err
		}

		// 检查状态
		if bucket.Get([]byte("expired")) != nil {
			return ErrLuckyMoneydExpired
		}

		// 已领取数量
		seq := bucket.Get([]byte("seq"))
		numReceived, err := strconv.Atoi(string(seq))
		if err != nil {
			return err
		}

		// 红包是否充足
		var base LuckyMoney
		jsb := bucket.Get([]byte("base"))
		if err = json.Unmarshal(jsb, &base); err != nil {
			return err
		}
		base.Normalization()

		if uint32(numReceived) >= base.Number {
			return ErrNothingLeft
		}

		// 红包是否激活
		if !base.Active {
			base.Active = true
		}

		// 是否重复领取
		usersBucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid, "users")
		if err != nil {
			return err
		}
		key := []byte(strconv.FormatInt(userID, 10))
		if usersBucket.Get(key) != nil {
			return ErrRepeatReceive
		}

		// 执行领取红包
		newSeq := numReceived + 1
		value, err = model.receiveLuckyMoney(tx, sid, newSeq, &LuckyMoneyUser{
			UserID:    userID,
			FirstName: firstName,
		})
		if err != nil {
			return err
		}
		base.Received = fmath.Add(base.Received, value)

		// 更新红包信息
		if jsb, err = json.Marshal(&base); err != nil {
			return err
		}
		if err = bucket.Put([]byte("base"), jsb); err != nil {
			return err
		}
		if err = usersBucket.Put(key, []byte("")); err != nil {
			return err
		}
		if err = bucket.Put([]byte("seq"), []byte(strconv.Itoa(newSeq))); err != nil {
			return err
		}

		// 添加用户历史
		if uint32(newSeq) >= base.Number {
			if err = model.moveToUserHistory(tx, base.SenderID, sid); err != nil {
				return err
			}
		}

		count = int(base.Number - uint32(newSeq))
		return nil
	})

	if err != nil {
		return nil, 0, err
	}
	return value, count, nil
}

// GetReceiveHistory 获取领取历史
func (model *LuckyMoneyModel) GetReceiveHistory(id uint64) ([]*LuckyMoneyHistory, error) {
	sid := strconv.FormatUint(id, 10)
	array := make([]*LuckyMoneyHistory, 0)
	err := storage.DB.View(func(tx *bolt.Tx) error {
		historyBucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid, "history")
		if err != nil {
			if err != storage.ErrNoBucket {
				return err
			}
			return nil
		}

		cursor := historyBucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			if v != nil {
				var item LuckyMoneyHistory
				if err = json.Unmarshal(v, &item); err != nil {
					return err
				}
				item.Normalization()

				if item.User == nil {
					return nil
				}
				array = append(array, &item)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return array, nil
}

// GetBestAndWorst 获取最佳红包
func (model *LuckyMoneyModel) GetBestAndWorst(id uint64) (*LuckyMoneyHistory, *LuckyMoneyHistory, error) {
	var best LuckyMoneyHistory
	var worst LuckyMoneyHistory
	sid := strconv.FormatUint(id, 10)
	err := storage.DB.View(func(tx *bolt.Tx) error {
		bucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid)
		if err != nil {
			return err
		}

		// 获取序列号
		bestSeq := bucket.Get([]byte("best"))
		worstSeq := bucket.Get([]byte("worst"))
		if worstSeq == nil || bestSeq == nil {
			return errors.New("nou found")
		}

		// 获取红包信息
		historyBucket, err := storage.GetBucketIfExists(tx, "luckymoney", sid, "history")
		if err != nil {
			return err
		}

		bestData := historyBucket.Get(bestSeq)
		worstData := historyBucket.Get(worstSeq)
		if bestData == nil || worstData == nil {
			return errors.New("nou found")
		}

		if err = json.Unmarshal(bestData, &best); err != nil {
			return err
		}
		if err = json.Unmarshal(worstData, &worst); err != nil {
			return err
		}

		best.Normalization()
		worst.Normalization()
		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	return &best, &worst, nil
}

// Foreach 遍历红包列表
func (model *LuckyMoneyModel) Foreach(startID uint64, callback func(*LuckyMoney)) error {
	var base LuckyMoney
	return storage.DB.View(func(tx *bolt.Tx) error {
		rootBucket, err := storage.GetBucketIfExists(tx, "luckymoney")
		if err != nil {
			return err
		}

		cursor := rootBucket.Cursor()
		seek := []byte(strconv.FormatUint(startID, 10))
		for k, v := cursor.Seek(seek); k != nil && v == nil; k, v = cursor.Next() {
			if bucket := rootBucket.Bucket(k); bucket != nil {
				jsb := bucket.Get([]byte("base"))
				if err = json.Unmarshal(jsb, &base); err != nil {
					continue
				}
				base.Normalization()

				if callback != nil {
					callback(&base)
				}
			}
		}
		return nil
	})
}

// Collection 获取用户红包
func (model *LuckyMoneyModel) Collection(userID int64, pending bool, offset, limit uint, reverse bool) ([]uint64, uint, error) {
	var sum uint
	typ := "pending"
	if !pending {
		typ = "history"
	}
	ids := make([]uint64, 0)
	key := strconv.FormatInt(userID, 10)
	err := storage.DB.View(func(tx *bolt.Tx) error {
		root, err := storage.GetBucketIfExists(tx, "luckymoney", typ)
		if err != nil {
			if err != storage.ErrNoBucket {
				return err
			}
			return nil
		}

		bucket := root.Bucket([]byte(key))
		if bucket == nil {
			return nil
		}

		sum = uint(bucket.Stats().KeyN)
		if offset >= sum {
			return nil
		}

		filter := func(idx uint, k, v []byte) bool {
			if v != nil {
				if idx >= offset {
					id, err := strconv.ParseUint(string(k), 10, 64)
					if err == nil {
						ids = append(ids, id)
						if len(ids) >= int(limit) {
							return false
						}
					}
				}
				idx++
			}
			return true
		}

		var idx uint
		cursor := bucket.Cursor()
		if reverse {
			for k, v := cursor.Last(); k != nil; k, v = cursor.Prev() {
				if !filter(idx, k, v) {
					break
				}
				idx++
			}
		} else {
			for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
				if !filter(idx, k, v) {
					break
				}
				idx++
			}
		}
		return nil
	})

	if err != nil {
		return nil, sum, err
	}
	return ids, sum, nil
}
