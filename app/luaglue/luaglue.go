package luaglue

import (
	"strconv"
	"time"

	"github.com/yuin/gopher-lua"
)

// LuaGlue Lua胶水
type LuaGlue struct {
	closed bool
	state  *lua.LState
}

// NewLuaGlue 创建实例
func NewLuaGlue() (*LuaGlue, error) {
	state := lua.NewState()
	state.PreloadModule("http", HttpLoader)
	state.PreloadModule("json", JsonLoader)
	if err := state.DoFile("scripts/main.lua"); err != nil {
		return nil, err
	}
	glue := LuaGlue{state: state}
	go glue.eventLoop()
	return &glue, nil
}

// 事件循环
func (glue *LuaGlue) eventLoop() {
	lasttime := time.Now()
	duration := 100 * time.Millisecond
	timer := time.NewTimer(duration)
	for {
		select {
		case <-timer.C:
			now := time.Now()
			glue.OnTick(now.Sub(lasttime).Seconds())
			lasttime = now
			timer.Reset(duration)
		}

		if glue.closed {
			break
		}
	}
}

// Close 释放资源
func (glue *LuaGlue) Close() {
	glue.state.Close()
	glue.closed = true
}

// OnTick 时钟事件
func (glue *LuaGlue) OnTick(delaytime float64) {
	fn := glue.state.GetGlobal("on_tick")
	if fn == nil {
		return
	}

	err := glue.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}, lua.LNumber(delaytime))
	if err != nil {
		return
	}
}

// ValidAddress 地址是否有效
func (glue *LuaGlue) ValidAddress(address string) bool {
	fn := glue.state.GetGlobal("valid_address")
	if fn == nil {
		return false
	}

	err := glue.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, lua.LString(address))
	if err != nil {
		return false
	}

	ret := glue.state.Get(-1)
	defer glue.state.Pop(1)
	if ret.Type() != lua.LTBool {
		return false
	}

	val := ret.(lua.LBool)
	return bool(val)
}

// DepositAddress 获取充值地址
func (glue *LuaGlue) DepositAddress(userID int64) (string, string) {
	fn := glue.state.GetGlobal("deposit_address")
	if fn == nil {
		return "", ""
	}

	err := glue.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    2,
		Protect: true,
	}, lua.LString(strconv.FormatInt(userID, 10)))
	if err != nil {
		return "", ""
	}

	address := ""
	addrRet := glue.state.Get(-2)
	if addrRet == nil || addrRet.Type() != lua.LTString {
		return "", ""
	}
	address = string(addrRet.(lua.LString))

	var memo string
	memoRet := glue.state.Get(-1)
	if memoRet != nil && memoRet.Type() == lua.LTString {
		memo = string(memoRet.(lua.LString))
	}
	return address, memo
}

// OnWithdraw 接收提现请求
func (glue *LuaGlue) OnWithdraw(to, symbol, amount string, id string) {
	fn := glue.state.GetGlobal("on_withdraw")
	if fn == nil {
		return
	}

	future := newFuture(glue.state, id)
	err := glue.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}, lua.LString(to), lua.LString(symbol), lua.LString(amount), future)
	if err != nil {
		return
	}
}

// ValidTransaction 交易是否有效
func (glue *LuaGlue) ValidTransaction(txid, from, to, symbol, amount, memo string) bool {
	fn := glue.state.GetGlobal("valid_transaction")
	if fn == nil {
		return false
	}

	err := glue.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, lua.LString(txid), lua.LString(from), lua.LString(to), lua.LString(symbol),
		lua.LString(amount), lua.LString(memo))
	if err != nil {
		return false
	}

	ret := glue.state.Get(-1)
	defer glue.state.Pop(1)
	if ret.Type() != lua.LTBool {
		return false
	}

	val := ret.(lua.LBool)
	return bool(val)
}
