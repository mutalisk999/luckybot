package fmath

import (
	"math/big"
)

var prec uint

func init() {
	prec = big.NewFloat(0).Prec()
}

// Prec 精度
func Prec() uint {
	return prec
}

// Add 相加
func Add(x *big.Float, y *big.Float) *big.Float {
	return big.NewFloat(0).Add(x, y)
}

// Sub 相减
func Sub(x *big.Float, y *big.Float) *big.Float {
	return big.NewFloat(0).Sub(x, y)
}

// Mul 相乘
func Mul(x *big.Float, y *big.Float) *big.Float {
	return big.NewFloat(0).Mul(x, y)
}

// Abs 取绝对值
func Abs(x *big.Float) *big.Float {
	return big.NewFloat(0).Abs(x)
}
