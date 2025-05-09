package utils

import (
	"github.com/shopspring/decimal"
	"math"
	"math/big"
)

func BigIntToFloat(num *big.Int, tokenDecimal int64) float64 {
	v, _ := decimal.NewFromString(num.String())
	decimalBase := decimal.NewFromFloat(math.Pow(10, float64(tokenDecimal)))
	return v.Div(decimalBase).Round(6).InexactFloat64()
}

func BigIntToInt(num *big.Int, tokenDecimal int64) int64 {
	v, _ := decimal.NewFromString(num.String())
	decimalBase := decimal.NewFromFloat(math.Pow(10, float64(tokenDecimal)))
	return v.Div(decimalBase).IntPart()
}
