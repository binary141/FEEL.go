package feel

import (
	"errors"
	"fmt"
	"strings"
)

func (binop Binop) Eval(intp *Interpreter) (any, error) {
	switch binop.Op {
	case "and":
		return binop.andOp(intp)
	case "or":
		return binop.orOp(intp)
	case "+":
		return binop.addOp(intp)
	case "-":
		return binop.subOp(intp)
	case "*":
		return binop.mulOp(intp)
	case "**":
		return binop.powOp(intp)
	case "/":
		return binop.divOp(intp)
	case "%":
		return binop.modOp(intp)
	case ">":
		return binop.compareGTOp(intp)
	case ">=":
		return binop.compareGEOp(intp)
	case "<":
		return binop.compareLTOp(intp)
	case "<=":
		return binop.compareLEOp(intp)
	case "!=":
		return binop.notEqalOp(intp)
	case "[]":
		return binop.indexAtOp(intp)
	case "=":
		return binop.equalOp(intp)
	case "in":
		return binop.inOp(intp)
	default:
		return nil, NewEvalError(-3000, "no such binary op", fmt.Sprintf("Binary op %s not exist or not supported", binop.Op))
	}
}

type evalNumbers func(a, b *Number) any
type evalStrings func(a, b string) any

func (binop Binop) numberOp(intp *Interpreter, en evalNumbers, op string) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	if _, ok := leftVal.(*NullValue); ok {
		return Null, nil
	}
	if _, ok := rightVal.(*NullValue); ok {
		return Null, nil
	}
	if leftNumber, ok := leftVal.(*Number); ok {
		if rightNumber, ok := rightVal.(*Number); ok {
			return en(leftNumber, rightNumber), nil
		}
	}
	return nil, NewEvalError(-3101, "invalid types", fmt.Sprintf("bad type in op, %T %s %T", leftVal, op, rightVal))
}

func CompareValues(leftVal, rightVal any) int {
	r, err := compareInterfaces(leftVal, rightVal)
	if err != nil {
		panic(err)
	} else {
		return r
	}
}

func compareInterfaces(leftVal, rightVal any) (int, error) {
	switch v := leftVal.(type) {
	case string:
		if rightString, ok := rightVal.(string); ok {
			return strings.Compare(v, rightString), nil
		}
	case *Number:
		if rightNumber, ok := rightVal.(*Number); ok {
			return v.Compare(*rightNumber), nil
		}
	case *NullValue:
		if _, ok := rightVal.(*NullValue); ok {
			return 0, nil
		}
	case bool:
		if rightBool, ok := rightVal.(bool); ok {
			if v == rightBool {
				return 0, nil
			} else if !v {
				return -1, nil
			} else {
				return 1, nil
			}
		}
	case HasTime:
		if rightHasTime, ok := rightVal.(HasTime); ok {
			if v.Time().Equal(rightHasTime.Time()) {
				return 0, nil
			} else if v.Time().Before(rightHasTime.Time()) {
				return -1, nil
			} else {
				return 1, nil
			}
		}
	case *FEELDuration:
		if rightDur, ok := rightVal.(*FEELDuration); ok && v.IsYearMonth() == rightDur.IsYearMonth() {
			var left, right int64
			if v.IsYearMonth() {
				left = v.TotalMonths()
				right = rightDur.TotalMonths()
			} else {
				left = int64(v.Duration())
				right = int64(rightDur.Duration())
			}
			if left == right {
				return 0, nil
			} else if left < right {
				return -1, nil
			} else {
				return 1, nil
			}
		}
	case []any:
		if rightArr, ok := rightVal.([]any); ok {
			return compareArrays(v, rightArr)
		}
	case map[string]any:
		if rightMap, ok := rightVal.(map[string]any); ok {
			return compareMaps(v, rightMap)
		}
	case *RangeValue:
		if rightRange, ok := rightVal.(*RangeValue); ok {
			if v.StartOpen != rightRange.StartOpen || v.EndOpen != rightRange.EndOpen {
				return 1, nil
			}
			cmpStart, err := compareInterfaces(v.Start, rightRange.Start)
			if err != nil {
				return 0, err
			}
			if cmpStart != 0 {
				return cmpStart, nil
			}
			return compareInterfaces(v.End, rightRange.End)
		}
	}
	return 0, NewEvalError(-3106, "invalid types", fmt.Sprintf("bad type in comparation, %T vs. %T", leftVal, rightVal))
}

func compareArrays(a, b []any) (int, error) {
	minSize := min(len(a), len(b))
	for i := 0; i < minSize; i++ {
		leftVal := a[i]
		rightVal := b[i]
		r, err := compareInterfaces(leftVal, rightVal)
		if err != nil {
			return 0, err
		}
		if r != 0 {
			return r, nil
		}
	}
	if len(a) == len(b) {
		return 0, nil
	} else if len(b) > len(a) {
		return -1, nil
	} else {
		return 1, nil
	}
}

func compareMaps(a, b map[string]any) (int, error) {
	if len(a) > len(b) {
		return 1, nil
	} else if len(a) < len(b) {
		return -1, nil
	}
	for k, leftVal := range a {
		if rightVal, ok := b[k]; ok {
			r, err := compareInterfaces(leftVal, rightVal)
			if err != nil {
				return 0, err
			}
			if r != 0 {
				return r, nil
			}
		} else {
			return 1, nil
		}
	}
	return 0, nil
}

func (binop Binop) compareValues(intp *Interpreter) (int, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return 0, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return 0, err
	}
	return compareInterfaces(leftVal, rightVal)
}

// orderingCompare evaluates operands for ordering comparisons (<, <=, >, >=).
// Per the FEEL spec, if either operand is null the result is null, not an error.
func (binop Binop) orderingCompare(intp *Interpreter) (int, bool, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return 0, false, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return 0, false, err
	}
	if _, ok := leftVal.(*NullValue); ok {
		return 0, true, nil
	}
	if _, ok := rightVal.(*NullValue); ok {
		return 0, true, nil
	}
	r, err := compareInterfaces(leftVal, rightVal)
	return r, false, err
}

func (binop Binop) typedOp(intp *Interpreter, es evalStrings, en evalNumbers, op string) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	if _, ok := leftVal.(*NullValue); ok {
		return Null, nil
	}
	if _, ok := rightVal.(*NullValue); ok {
		return Null, nil
	}

	switch v := leftVal.(type) {
	case string:
		if es != nil {
			if rightString, ok := rightVal.(string); ok {
				return es(v, rightString), nil
			}
		}
	case *Number:
		if en != nil {
			if rightNumber, ok := rightVal.(*Number); ok {
				return en(v, rightNumber), nil
			}
		}
	case *FEELDatetime:
		if op == "+" {
			if rightDur, ok := rightVal.(*FEELDuration); ok {
				return v.Add(rightDur), nil
			}
		} else if op == "-" {
			if rightDur, ok := rightVal.(*FEELDuration); ok {
				return v.Add(rightDur.Negative()), nil
			} else if rightTime, ok := rightVal.(HasTime); ok {
				return v.Sub(rightTime), nil
			}
		}
	case *FEELDate:
		if op == "+" {
			if rightDur, ok := rightVal.(*FEELDuration); ok {
				return v.Add(rightDur), nil
			}
		} else if op == "-" {
			if rightDur, ok := rightVal.(*FEELDuration); ok {
				return v.Add(rightDur.Negative()), nil
			} else if rightDate, ok := rightVal.(*FEELDate); ok {
				return v.Sub(rightDate), nil
			}
		}
	case *FEELTime:
		if op == "+" {
			if rightDur, ok := rightVal.(*FEELDuration); ok {
				return v.Add(rightDur), nil
			}
		} else if op == "-" {
			if rightDur, ok := rightVal.(*FEELDuration); ok {
				return v.Add(rightDur.Negative()), nil
			} else if rightTime, ok := rightVal.(*FEELTime); ok {
				return v.Sub(rightTime), nil
			}
		}
	case *FEELDuration:
		if rightDur, ok := rightVal.(*FEELDuration); ok {
			if op == "+" {
				result, err := v.Add(rightDur)
				if err != nil {
					return Null, nil
				}
				return result, nil
			} else if op == "-" {
				result, err := v.Sub(rightDur)
				if err != nil {
					return Null, nil
				}
				return result, nil
			}
		}
		if op == "+" {
			if rightDate, ok := rightVal.(*FEELDate); ok {
				return rightDate.Add(v), nil
			}
			if rightTime, ok := rightVal.(*FEELTime); ok {
				return rightTime.Add(v), nil
			}
			if rightDT, ok := rightVal.(*FEELDatetime); ok {
				return rightDT.Add(v), nil
			}
		}
	}
	//return nil, NewEvalError(-3101, "invalid types", fmt.Sprintf("bad types in op, %s %s %s", typeName(leftVal), op, typeName(rightVal)))
	return nil, NewErrBadOp(typeName(leftVal), op, typeName(rightVal))
}

func (binop Binop) addOp(intp *Interpreter) (any, error) {
	return binop.typedOp(
		intp,
		func(a, b string) any { return a + b },
		func(a, b *Number) any { return a.Add(b) },
		"+",
	)
}

func (binop Binop) subOp(intp *Interpreter) (any, error) {
	return binop.typedOp(
		intp,
		nil,
		func(a, b *Number) any { return a.Sub(b) },
		"-")
}

func (binop Binop) mulOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	if _, ok := leftVal.(*NullValue); ok {
		return Null, nil
	}
	if _, ok := rightVal.(*NullValue); ok {
		return Null, nil
	}
	if leftNumber, ok := leftVal.(*Number); ok {
		if rightNumber, ok := rightVal.(*Number); ok {
			return leftNumber.Mul(rightNumber), nil
		}
		if rightDur, ok := rightVal.(*FEELDuration); ok {
			result, err := rightDur.MulNumber(leftNumber)
			if err != nil {
				return Null, nil
			}
			return result, nil
		}
	}
	if leftDur, ok := leftVal.(*FEELDuration); ok {
		if rightNumber, ok := rightVal.(*Number); ok {
			result, err := leftDur.MulNumber(rightNumber)
			if err != nil {
				return Null, nil
			}
			return result, nil
		}
	}
	return nil, NewErrBadOp(typeName(leftVal), "*", typeName(rightVal))
}

func (binop Binop) powOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	leftNumber, leftOk := leftVal.(*Number)
	rightNumber, rightOk := rightVal.(*Number)
	if !leftOk || !rightOk {
		return Null, nil
	}
	return leftNumber.Pow(rightNumber), nil
}

func (binop Binop) divOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	if _, ok := leftVal.(*NullValue); ok {
		return Null, nil
	}
	if _, ok := rightVal.(*NullValue); ok {
		return Null, nil
	}
	if leftNumber, ok := leftVal.(*Number); ok {
		if rightNumber, ok := rightVal.(*Number); ok {
			if rightNumber.IsZero() {
				return Null, nil
			}
			return leftNumber.FloatDiv(rightNumber), nil
		}
	}
	if leftDur, ok := leftVal.(*FEELDuration); ok {
		if rightNumber, ok := rightVal.(*Number); ok {
			result, err := leftDur.DivNumber(rightNumber)
			if err != nil {
				return Null, nil
			}
			return result, nil
		}
		if rightDur, ok := rightVal.(*FEELDuration); ok {
			result, err := leftDur.DivDuration(rightDur)
			if err != nil {
				return Null, nil
			}
			return result, nil
		}
	}
	return nil, NewErrBadOp(typeName(leftVal), "/", typeName(rightVal))
}

func (binop Binop) compareGTOp(intp *Interpreter) (any, error) {
	r, isNull, err := binop.orderingCompare(intp)
	if err != nil {
		return false, err
	} else if isNull {
		return Null, nil
	} else {
		return r > 0, nil
	}
}

func (binop Binop) compareGEOp(intp *Interpreter) (any, error) {
	r, isNull, err := binop.orderingCompare(intp)
	if err != nil {
		return false, err
	} else if isNull {
		return Null, nil
	} else {
		return r >= 0, nil
	}
}

func (binop Binop) compareLTOp(intp *Interpreter) (any, error) {
	r, isNull, err := binop.orderingCompare(intp)
	if err != nil {
		return false, err
	} else if isNull {
		return Null, nil
	} else {
		return r < 0, nil
	}
}

func (binop Binop) compareLEOp(intp *Interpreter) (any, error) {
	r, isNull, err := binop.orderingCompare(intp)
	if err != nil {
		return false, err
	} else if isNull {
		return Null, nil
	} else {
		return r <= 0, nil
	}
}

func (binop Binop) equalOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	_, leftNull := leftVal.(*NullValue)
	_, rightNull := rightVal.(*NullValue)
	// null compared against a known non-null value is a definite
	// inequality; only a genuine type mismatch between two non-null
	// values of incompatible types is "unknown" (null).
	if leftNull != rightNull {
		return false, nil
	}
	r, err := compareInterfaces(leftVal, rightVal)
	if err != nil {
		var evalError *EvalError
		if errors.As(err, &evalError) && evalError.Code == -3106 {
			return Null, nil
		}
		return false, err
	}
	return r == 0, nil
}

func (binop Binop) notEqalOp(intp *Interpreter) (any, error) {
	r, err := binop.compareValues(intp)
	if err != nil {
		var evalError *EvalError
		if errors.As(err, &evalError) && evalError.Code == -3106 {
			// type mismatch
			return false, nil
		}
		return false, err
	} else {
		return r != 0, nil
	}
}

func (binop Binop) modOp(intp *Interpreter) (any, error) {
	return binop.numberOp(
		intp,
		func(a, b *Number) any {
			if b.IsZero() {
				return Null
			}
			return a.IntMod(b)
		},
		"%")
}

// circuit break operators. Per FEEL's three-valued logic, only actual
// booleans participate as true/false; anything else (null, a number, a
// string, ...) is "unknown" and only affects the result when it isn't
// already decided by a false (and) / true (or) operand.
func (binop Binop) andOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	if lb, ok := leftVal.(bool); ok && !lb {
		return false, nil
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	if rb, ok := rightVal.(bool); ok && !rb {
		return false, nil
	}
	lb, lok := leftVal.(bool)
	rb, rok := rightVal.(bool)
	if lok && rok {
		return lb && rb, nil
	}
	return Null, nil
}

func (binop Binop) orOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	if lb, ok := leftVal.(bool); ok && lb {
		return true, nil
	}
	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	if rb, ok := rightVal.(bool); ok && rb {
		return true, nil
	}
	lb, lok := leftVal.(bool)
	rb, rok := rightVal.(bool)
	if lok && rok {
		return lb || rb, nil
	}
	return Null, nil
}

func (binop Binop) indexAtOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	// A speculative, scope-free evaluation of the right side just to see
	// whether this is a numeric index (`list[1]`) or a filter predicate
	// (`list[item.a >= 2]`). Filter predicates typically reference "item"
	// or other per-element bindings that don't exist yet here, so errors
	// are expected and simply mean "not a number index" — the predicate
	// gets evaluated for real, per element, in filterList below.
	rightVal, rightErr := binop.Right.Eval(intp)
	if rightErr != nil {
		rightVal = nil
	}
	// filterList evaluates binop.Right per element of an implicit list
	// (binding "item" to the current element, plus its fields when it's a
	// context) and returns the elements for which the predicate is true.
	filterList := func(elems []any) (any, error) {
		var result []any
		for _, elem := range elems {
			scope := Scope{"item": elem}
			if mapElem, ok := elem.(map[string]any); ok {
				for k, val := range mapElem {
					scope[k] = val
				}
			}
			intp.Push(scope)
			r, err := binop.Right.Eval(intp)
			intp.Pop()
			if err != nil {
				continue
			}
			if boolValue(r) {
				result = append(result, elem)
			}
		}
		if result == nil {
			return []any{}, nil
		}
		return result, nil
	}

	switch v := leftVal.(type) {
	case []any:
		if nRight, ok := rightVal.(*Number); ok {
			at := nRight.Int()
			if at < 0 {
				at = len(v) + at + 1
			}
			if at <= 0 || at > len(v) {
				return Null, nil
			}
			return v[at-1], nil
		}
		return filterList(v)
	case map[string]any:
		if strRight, ok := rightVal.(string); ok {
			if elem, ok := v[strRight]; ok {
				return elem, nil
			} else {
				//return nil, NewEvalError(-3201, "key not found")
				return nil, NewErrKeyNotFound(strRight)
			}
		} else if nRight, ok := rightVal.(*Number); ok {
			// A context behaves as an implicit singleton list: ctx[1] == ctx.
			if nRight.Int() == 1 {
				return leftVal, nil
			}
			return Null, nil
		} else {
			//return nil, NewEvalError(-3200, "non string index")
			return nil, NewErrIndex("non string index")
		}
	default:
		// A scalar behaves as an implicit singleton list: x[1] == x.
		if nRight, ok := rightVal.(*Number); ok {
			at := nRight.Int()
			if at < 0 {
				at = 1 + at + 1
			}
			if at == 1 {
				return leftVal, nil
			}
			return Null, nil
		}
		return filterList([]any{leftVal})
	}
}

func (binop Binop) inOp(intp *Interpreter) (any, error) {
	leftVal, err := binop.Left.Eval(intp)
	if err != nil {
		return nil, err
	}

	// Bind ? = leftVal so unary tests (<val, <=val, …) resolve correctly.
	intp.Push(Scope{"?": leftVal})
	defer intp.Pop()

	rightVal, err := binop.Right.Eval(intp)
	if err != nil {
		return nil, err
	}
	switch rv := rightVal.(type) {
	case *RangeValue:
		ok, err := rv.Contains(leftVal)
		if err != nil {
			return Null, nil
		}
		return ok, nil
	case []any:
		for _, kv := range rv {
			if rangeVal, ok := kv.(*RangeValue); ok {
				if inside, err := rangeVal.Contains(leftVal); err == nil && inside {
					return true, nil
				}
			} else if r, err := compareInterfaces(leftVal, kv); err == nil && r == 0 {
				return true, nil
			}
		}
		return false, nil
	case bool:
		// Result of a unary test expression (e.g. ? < 5 evaluated to a bool).
		return rv, nil
	default:
		// Scalar: equality check (e.g. 1 in 1 → true).
		r, err := compareInterfaces(leftVal, rightVal)
		if err != nil {
			return Null, nil
		}
		return r == 0, nil
	}
}
