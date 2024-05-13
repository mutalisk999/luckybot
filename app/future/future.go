package future

type Future struct {
	id string
	ch chan result
}

// 处理结果
type result struct {
	err  error
	txid string
}

// 创建Future
func newFuture(id string) *Future {
	ch := make(chan result)
	return &Future{ch: ch, id: id}
}

// ID 获取ID
func (f *Future) ID() string {
	return f.id
}

// GetResult 获取结果
func (f *Future) GetResult() (string, error) {
	r := <-f.ch
	return r.txid, r.err
}

// SetResult 设置结果
func (f *Future) SetResult(txid string, err error) {
	f.ch <- result{txid: txid, err: err}
}
