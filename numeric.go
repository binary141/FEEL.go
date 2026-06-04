package feel

// FEEL numbers follow IEEE 754-2008 Decimal 128: 34 significant digits, half-even
// rounding, exponent range [-6143, 6144]. This matches Java's BigDecimal with
// MathContext.DECIMAL128. https://kiegroup.github.io/dmn-feel-handbook/#number
import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/cockroachdb/apd/v3"
)

// decimal128Context is the authoritative context for all FEEL arithmetic.
var decimal128Context = apd.Context{
	Precision:   34,
	Rounding:    apd.RoundHalfEven,
	MaxExponent: 6144,
	MinExponent: -6143,
	Traps:       apd.DefaultTraps,
}

// displayContext rounds to 34 significant digits (DECIMAL128 full spec) for display/output.
var displayContext = apd.Context{
	Precision:   34,
	Rounding:    apd.RoundHalfEven,
	MaxExponent: 6144,
	MinExponent: -6143,
}

var (
	ErrParseNumber = errors.New("fail to parse number")
)

type Number struct {
	v *apd.Decimal
}

func NewNumber(strn string) *Number {
	d := new(apd.Decimal)
	if _, _, err := d.SetString(strn); err != nil {
		d = new(apd.Decimal)
	}
	return &Number{v: d}
}

func NewNumberFromInt64(input int64) *Number {
	return &Number{v: apd.New(input, 0)}
}

func NewNumberFromFloat(input float64) *Number {
	d := new(apd.Decimal)
	if _, err := d.SetFloat64(input); err != nil {
		d = new(apd.Decimal)
	}
	return &Number{v: d}
}

func ParseNumberWithErr(v any) (*Number, error) {
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

func N(v any) *Number {
	n, err := ParseNumberWithErr(v)
	if err != nil {
		panic(err)
	}
	return n
}

func (number Number) Int64() int64 {
	// Modf truncates toward zero so 3.8 → 3, -3.8 → -3, matching FEEL semantics.
	integ := new(apd.Decimal)
	frac := new(apd.Decimal)
	number.v.Modf(integ, frac)
	i, _ := integ.Int64()
	return i
}

func (number Number) Int() int {
	return int(number.Int64())
}

func (number Number) Float64() float64 {
	f, _ := number.v.Float64()
	return f
}

func (number *Number) Add(other *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.Add(result, number.v, other.v) //nolint:errcheck
	return &Number{v: result}
}

func (number *Number) Sub(other *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.Sub(result, number.v, other.v) //nolint:errcheck
	return &Number{v: result}
}

func (number *Number) Mul(other *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.Mul(result, number.v, other.v) //nolint:errcheck
	return &Number{v: result}
}

func (number *Number) Cmp(other *Number) int {
	return number.v.Cmp(other.v)
}

func (number *Number) IntDiv(other *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.QuoInteger(result, number.v, other.v) //nolint:errcheck
	return &Number{v: result}
}

func (number *Number) FloatDiv(other *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.Quo(result, number.v, other.v) //nolint:errcheck
	return &Number{v: result}
}

func (number *Number) Pow(exp *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.Pow(result, number.v, exp.v) //nolint:errcheck
	return &Number{v: result}
}

func (number *Number) IsZero() bool {
	return number.v.IsZero()
}

func (number *Number) IntMod(other *Number) *Number {
	result := new(apd.Decimal)
	decimal128Context.Rem(result, number.v, other.v) //nolint:errcheck
	return &Number{v: result}
}

func (number Number) Equal(other Number) bool {
	return number.Compare(other) == 0
}

func (number Number) Compare(other Number) int {
	return number.v.Cmp(other.v)
}

func (number Number) CompareRounded(other Number, decimalPlaces int32) int {
	a := new(apd.Decimal)
	b := new(apd.Decimal)
	displayContext.Quantize(a, number.v, -decimalPlaces) //nolint:errcheck
	displayContext.Quantize(b, other.v, -decimalPlaces)  //nolint:errcheck
	return a.Cmp(b)
}

func (number Number) String() string {
	rounded := new(apd.Decimal)
	displayContext.Round(rounded, number.v) //nolint:errcheck
	s := rounded.Text('f')
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func (number Number) MarshalJSON() ([]byte, error) {
	return json.Marshal(number.String())
}

var Zero = N(0)
