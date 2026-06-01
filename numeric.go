package feel

// spec on FEEL's number is https://kiegroup.github.io/dmn-feel-handbook/#number
import (
	"encoding/json"
	"errors"
	"math"
	"math/big"
)

const (
	Prec = 34 * 8
)

var (
	ErrParseNumber = errors.New("fail to parse number")
)

type Number struct {
	v *big.Float
}

func NewNumber(strn string) *Number {
	v := new(big.Float)
	v.SetPrec(Prec).SetString(strn)
	return &Number{v: v}
}

func NewNumberFromInt64(input int64) *Number {
	v := new(big.Float)
	v.SetPrec(200).SetInt64(input)
	return &Number{v: v}
}

func NewNumberFromFloat(input float64) *Number {
	v := new(big.Float)
	v.SetPrec(200).SetFloat64(input)
	return &Number{v: v}
}

func ParseNumberWithErr(v interface{}) (*Number, error) {
	switch vv := v.(type) {
	case int:
		return NewNumberFromInt64(int64(vv)), nil
	case int64:
		return NewNumberFromInt64(vv), nil
	case float64:
		return NewNumberFromFloat(vv), nil
	case string:
		return NewNumber(vv), nil
	case *Number:
		return vv, nil
	default:
		return nil, ErrParseNumber
	}
}

func N(v interface{}) *Number {
	n, err := ParseNumberWithErr(v)
	if err != nil {
		panic(err)
	}
	return n
}

func (number Number) Int64() int64 {
	i64v, _ := number.v.Int64()
	return i64v
}

func (number Number) Int() int {
	return int(number.Int64())
}

func (number Number) Float64() float64 {
	f64v, _ := number.v.Float64()
	return f64v
}

func (number *Number) Add(other *Number) *Number {
	newv := new(big.Float)
	newv.SetPrec(Prec).Add(number.v, other.v)
	return &Number{v: newv}
}

func (number *Number) Sub(other *Number) *Number {
	newv := new(big.Float)
	newv.SetPrec(Prec).Sub(number.v, other.v)
	return &Number{v: newv}
}

func (number *Number) Mul(other *Number) *Number {
	newv := new(big.Float)
	newv.SetPrec(Prec).Mul(number.v, other.v)
	return &Number{v: newv}
}

func (number *Number) Cmp(other *Number) int {
	return number.v.Cmp(other.v)
}

func (number *Number) IntDiv(other *Number) *Number {
	newv := new(big.Int)
	a, _ := number.v.Int(nil)
	b, _ := other.v.Int(nil)
	newv.Div(a, b)
	newf := new(big.Float)
	newf.SetPrec(Prec).SetInt(newv)
	return &Number{v: newf}
}

func (number *Number) FloatDiv(other *Number) *Number {
	newf := new(big.Float)
	newf.SetPrec(Prec).Quo(number.v, other.v)
	return &Number{v: newf}
}

func (number *Number) Pow(exp *Number) *Number {
	if exp.v.IsInt() {
		n, _ := exp.v.Int64()
		return number.powInt(n)
	}
	a, _ := number.v.Float64()
	b, _ := exp.v.Float64()
	newf := new(big.Float)
	newf.SetPrec(Prec).SetFloat64(math.Pow(a, b))
	return &Number{v: newf}
}

func (number *Number) powInt(n int64) *Number {
	result := new(big.Float).SetPrec(Prec).SetInt64(1)
	base := new(big.Float).SetPrec(Prec).Set(number.v)
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		if n&1 == 1 {
			result.SetPrec(Prec).Mul(result, base)
		}
		base.SetPrec(Prec).Mul(base, base)
		n >>= 1
	}
	if negative {
		one := new(big.Float).SetPrec(Prec).SetInt64(1)
		result.SetPrec(Prec).Quo(one, result)
	}
	return &Number{v: result}
}

func (number *Number) IsZero() bool {
	return number.v.Sign() == 0
}

func (number *Number) IntMod(other *Number) *Number {
	newv := new(big.Int)
	a, _ := number.v.Int(nil)
	b, _ := other.v.Int(nil)
	newv.Mod(a, b)
	newf := new(big.Float)
	newf.SetPrec(Prec).SetInt(newv)
	return &Number{v: newf}
}

func (number Number) Equal(other Number) bool {
	return number.Compare(other) == 0
}

func (number Number) Compare(other Number) int {
	return number.v.Cmp(other.v)
}

func (number Number) String() string {
	//return number.v.String()
	return number.v.Text('f', 18)
}

func (number Number) MarshalJSON() ([]byte, error) {
	if f32v, acc := number.v.Float32(); acc == big.Exact {
		return json.Marshal(f32v)
	}
	return json.Marshal(number.String())
}

var Zero = N(0)
