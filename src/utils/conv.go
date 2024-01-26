package utils

import (
	"math"
	"math/big"
)

// floatToDecN converts a floating point number to a decimal-n,
// i.e., to num*10^decN
func FloatToDecN(num float64, decN uint8) *big.Int {
	// Convert the floating-point number to a big.Float
	floatNumber := new(big.Float).SetFloat64(num)

	// Multiply the floatNumber by 10^n
	multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decN)), nil))
	result := new(big.Float).Mul(floatNumber, multiplier)

	// Convert the result to a big.Int
	intResult, _ := result.Int(nil)
	return intResult
}

// DecNToFloat converts a decimal N number to
// the corresponding float number
func DecNToFloat(num *big.Int, decN uint8) float64 {
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decN)), nil))
	numf := new(big.Float).SetInt(num)
	smallFloat := new(big.Float).Quo(numf, divisor)
	f, _ := smallFloat.Float64()
	return f
}

// ABDKToDecN converts a 64.64 ABDK fixed point
// number into a decimal N number
func ABDKToDecN(num *big.Int, decN uint8) *big.Int {
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decN)), nil)
	numD := new(big.Int).Mul(num, multiplier)
	c64 := new(big.Int).Exp(big.NewInt(2), big.NewInt(64), nil)
	res := new(big.Int).Div(numD, c64)
	return res
}

func ABDKToFloat(num *big.Int) float64 {
	c64 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(64), nil))
	numf := new(big.Float).SetInt(num)
	res := new(big.Float).Quo(numf, c64)
	f, _ := res.Float64()
	return f
}

// Ratio calculates the ratio of two big int that are "similar" as float
func Ratio(x *big.Int, y *big.Int) float64 {
	xFloat := new(big.Float).SetInt(x)
	yFloat := new(big.Float).SetInt(y)
	resultFloat := new(big.Float).Quo(xFloat, yFloat)
	result, _ := resultFloat.Float64()
	return result
}

// DecNTimesFloat multiplies a decimal-N number
// with a fraction m>0 in decimal system. One application
// is to get a relative share (e.g., 1%) of the
// decimal-n number in the same decimal-n format
func DecNTimesFloat(num *big.Int, m float64, precision int) *big.Int {
	if m == 0 {
		return big.NewInt(int64(0))
	}
	v := math.Pow(10, float64(precision))
	mInt := int64(m * v)
	mIntB := big.NewInt(mInt)
	vB := big.NewInt(int64(v))
	resPow := new(big.Int).Mul(num, mIntB)
	res := new(big.Int).Div(resPow, vB)
	return res
}
