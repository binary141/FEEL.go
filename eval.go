package feel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// values

type NullValue struct {
}

func (v NullValue) Equal(other NullValue) bool {
	return true
}

func (v NullValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(nil)
}

var Null = &NullValue{}

func boolValue(condVal any) bool {
	switch v := condVal.(type) {
	case *NullValue:
		return false
	case int64:
		return v != 0
	case float64:
		return v != 0.0
	case *Number:
		return !v.Equal(*Zero)
	case bool:
		return v
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return v != nil
	}
}

func typeName(a any) string {
	switch a.(type) {
	case int64:
		return "number"
	case float64:
		return "number"
	case *Number:
		return "number"
	case bool:
		return "bool"
	case string:
		return "string"
	case []any:
		return "list"
	case map[string]any:
		return "context"
	case *NullValue:
		return "null"
	case *FEELDate:
		return "date"
	case *FEELTime:
		return "time"
	case *FEELDatetime:
		return "datetime"
	case *FEELDuration:
		return "duration"
	case *RangeValue:
		return "range"
	case *NativeFun:
		return "function"
	case *FunDef:
		return "function"
	case *Macro:
		return "function"
	default:
		return "unknown"
	}
}

func normalizeValue(v any) any {
	switch vv := v.(type) {
	case int:
		return NewNumberFromInt64(int64(vv))
	case int64:
		return NewNumberFromInt64(vv)
	case float64:
		return NewNumberFromFloat(vv)
	default:
		return vv
	}
}

func (scope Scope) normalizeScope() Scope {
	newScp := make(Scope)
	for key, value := range scope {
		newScp[key] = normalizeValue(value)
	}
	return newScp
}

// intepreter
func NewIntepreter() *Interpreter {
	intp := &Interpreter{}
	intp.PushEmpty()
	return intp
}

func (intp Interpreter) String() string {
	return "interpreter"
}

func (intp Interpreter) Len() int {
	return len(intp.ScopeStack)
}

func (intp *Interpreter) Push(scp Scope) {
	intp.ScopeStack = append(intp.ScopeStack, scp.normalizeScope())
}

func (intp *Interpreter) PushEmpty() {
	vars := make(Scope)
	intp.Push(vars)
}

func (intp *Interpreter) Pop() Scope {
	if intp.Len() > 0 {
		top := intp.ScopeStack[len(intp.ScopeStack)-1]
		intp.ScopeStack = intp.ScopeStack[:len(intp.ScopeStack)-1]
		return top
	}
	return nil
}

// resolve a name from the top of scopestack to bottom
func (intp Interpreter) Resolve(name string) (any, bool) {
	for at := len(intp.ScopeStack) - 1; at >= 0; at-- {
		if v, ok := intp.ScopeStack[at][name]; ok {
			return v, true
		}
	}
	if prelude, ok := GetPrelude().Resolve(name); ok {
		return prelude, ok
	}
	return nil, false
}

// resolve the name and set to new value
func (intp Interpreter) Set(name string, value any) bool {
	for at := len(intp.ScopeStack) - 1; at >= 0; at-- {
		if _, ok := intp.ScopeStack[at][name]; ok {
			intp.ScopeStack[at][name] = value
			return true
		}
	}
	return false
}

// bind the value to the name of current scope
func (intp *Interpreter) Bind(name string, value any) {
	if intp.Len() > 0 {
		intp.ScopeStack[intp.Len()-1][name] = normalizeValue(value)
	} else {
		panic("empty stack")
	}
}

// Node's eval functions

// Evaluate Number node
func (n NumberNode) Eval(intp *Interpreter) (any, error) {
	return NewNumber(n.Value), nil
}

// Evaluate bool node
func (node BoolNode) Eval(intp *Interpreter) (any, error) {
	return node.Value, nil
}

func (node NullNode) Eval(intp *Interpreter) (any, error) {
	return Null, nil
}

func (node StringNode) Eval(intp *Interpreter) (any, error) {
	return node.Content(), nil
}

func (node TemporalNode) Eval(intp *Interpreter) (any, error) {
	return ParseTemporalValue(node.Content())
}

func (node Var) Eval(intp *Interpreter) (any, error) {
	if v, ok := intp.Resolve(node.Name); ok {
		return v, nil
	} else {
		//return nil, NewErrKeyNotFound(node.Name)
		return Null, nil
	}
}

func (node BetweenExpr) Eval(intp *Interpreter) (any, error) {
	val, err := node.Value.Eval(intp)
	if err != nil {
		return nil, err
	}
	lower, err := node.Lower.Eval(intp)
	if err != nil {
		return nil, err
	}
	upper, err := node.Upper.Eval(intp)
	if err != nil {
		return nil, err
	}
	if _, isNull := val.(*NullValue); isNull {
		return Null, nil
	}
	if _, isNull := lower.(*NullValue); isNull {
		return Null, nil
	}
	if _, isNull := upper.(*NullValue); isNull {
		return Null, nil
	}
	lowerCmp, err := compareInterfaces(lower, val)
	if err != nil {
		return Null, nil
	}
	upperCmp, err := compareInterfaces(val, upper)
	if err != nil {
		return Null, nil
	}
	return lowerCmp <= 0 && upperCmp <= 0, nil
}

var typeNameAliases = map[string]string{
	"date and time": "datetime",
	"boolean":       "bool",
}

func rangeElementTypeName(rv *RangeValue) string {
	if rv.elementType != "" {
		return rv.elementType
	}
	switch v := rv.Start.(type) {
	case *Number:
		return "range<number>"
	case string:
		return "range<string>"
	case *FEELDate:
		return "range<date>"
	case *FEELDatetime:
		return "range<date and time>"
	case *FEELTime:
		return "range<time>"
	case *FEELDuration:
		if v.IsYearMonth() {
			return "range<years and months duration>"
		}
		return "range<days and time duration>"
	default:
		return "range"
	}
}

func (node InstanceOfNode) Eval(intp *Interpreter) (any, error) {
	val, err := node.Value.Eval(intp)
	if err != nil {
		return nil, err
	}
	return matchesInstanceOfType(val, node.TypeName), nil
}

// splitTopLevel splits s on sep, ignoring occurrences of sep nested inside
// "<...>" (used for context<field: type, ...> and list<type> descriptors
// produced by parseInstanceOfTypeExpr).
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// matchesContextType checks val against a "field: type, field: type, ..."
// descriptor (the contents of a context<...> instance-of type). Extra
// fields present on val but not listed in the type are allowed; every
// listed field must be present on val with a matching type.
func matchesContextType(val any, inner string) bool {
	m, ok := val.(map[string]any)
	if !ok {
		return false
	}
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return true
	}
	for _, part := range splitTopLevel(inner, ',') {
		idx := strings.Index(part, ":")
		if idx < 0 {
			return false
		}
		name := strings.TrimSpace(part[:idx])
		fieldType := strings.TrimSpace(part[idx+1:])
		fv, exists := m[name]
		if !exists {
			return false
		}
		// A null property value conforms to any declared field type: DMN
		// context conformance treats a missing/null value as compatible.
		if _, isNull := fv.(*NullValue); isNull {
			continue
		}
		if !matchesInstanceOfType(fv, fieldType) {
			return false
		}
	}
	return true
}

// matchesInstanceOfType implements the "instance of" operator against a
// type descriptor string as produced by parseInstanceOfTypeExpr.
func matchesInstanceOfType(val any, expected string) bool {
	if expected == "Any" {
		_, isNull := val.(*NullValue)
		return !isNull
	}
	if strings.HasPrefix(expected, "range<") {
		rv, ok := val.(*RangeValue)
		if !ok {
			return false
		}
		return rangeElementTypeName(rv) == expected
	}
	if expected == "years and months duration" || expected == "days and time duration" {
		dur, ok := val.(*FEELDuration)
		if !ok {
			return false
		}
		return dur.IsYearMonth() == (expected == "years and months duration")
	}
	if strings.HasPrefix(expected, "list<") && strings.HasSuffix(expected, ">") {
		lst, ok := val.([]any)
		if !ok {
			return false
		}
		inner := expected[len("list<") : len(expected)-1]
		for _, item := range lst {
			if !matchesInstanceOfType(item, inner) {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(expected, "context<") && strings.HasSuffix(expected, ">") {
		return matchesContextType(val, expected[len("context<"):len(expected)-1])
	}
	// function<...>->...: FEEL.go has no structural function-signature
	// checking, so only the outer kind is checked.
	if idx := strings.IndexAny(expected, "<"); idx >= 0 {
		expected = expected[:idx]
	} else if idx := strings.Index(expected, " -> "); idx >= 0 {
		expected = expected[:idx]
	}
	if canonical, ok := typeNameAliases[expected]; ok {
		expected = canonical
	}
	return typeName(val) == expected
}

// materializeIterationList resolves a "for"/"some"/"every" iteration
// source into a concrete list of values. Alongside plain lists, FEEL
// supports iterating directly over a numeric or date range endpoint pair
// (e.g. "for i in 2..4 return i", ascending or descending, step 1/1 day,
// inclusive of both endpoints).
func materializeIterationList(val any) ([]any, error) {
	if lst, ok := val.([]any); ok {
		return lst, nil
	}
	rv, ok := val.(*RangeValue)
	if !ok {
		return nil, NewErrTypeMismatch("list")
	}
	switch start := rv.Start.(type) {
	case *Number:
		end, ok := rv.End.(*Number)
		if !ok {
			return nil, NewErrTypeMismatch("list")
		}
		if !rv.iterationRange && start.Cmp(end) > 0 {
			return nil, errors.New("descending range literal is invalid")
		}
		one := N(1)
		var out []any
		if start.Cmp(end) <= 0 {
			for n := start; n.Cmp(end) <= 0; n = n.Add(one) {
				out = append(out, n)
			}
		} else {
			for n := start; n.Cmp(end) >= 0; n = n.Sub(one) {
				out = append(out, n)
			}
		}
		return out, nil
	case *FEELDate:
		end, ok := rv.End.(*FEELDate)
		if !ok {
			return nil, NewErrTypeMismatch("list")
		}
		if !rv.iterationRange && start.Date().After(end.Date()) {
			return nil, errors.New("descending range literal is invalid")
		}
		oneDay := &FEELDuration{Days: 1}
		var out []any
		if !start.Date().After(end.Date()) {
			for d := start; !d.Date().After(end.Date()); d = d.Add(oneDay) {
				out = append(out, d)
			}
		} else {
			negOneDay := oneDay.Negative()
			for d := start; !d.Date().Before(end.Date()); d = d.Add(negOneDay) {
				out = append(out, d)
			}
		}
		return out, nil
	default:
		return nil, NewErrTypeMismatch("list")
	}
}

func (node NegateNode) Eval(intp *Interpreter) (any, error) {
	v, err := node.Operand.Eval(intp)
	if err != nil {
		return nil, err
	}
	switch vv := v.(type) {
	case *Number:
		return vv.Neg(), nil
	case *FEELDuration:
		return vv.Negative(), nil
	default:
		zero := valueNode{value: N(0), textRange: node.textRange}
		operand := valueNode{value: v, textRange: node.textRange}
		return (Binop{Op: "-", Left: zero, Right: operand, textRange: node.textRange}).Eval(intp)
	}
}

func (node RangeNode) Eval(intp *Interpreter) (any, error) {
	startVal, err := node.Start.Eval(intp)
	if err != nil {
		return nil, err
	}
	endVal, err := node.End.Eval(intp)
	if err != nil {
		return nil, err
	}
	return &RangeValue{
		Start:          startVal,
		StartOpen:      node.StartOpen,
		End:            endVal,
		EndOpen:        node.EndOpen,
		iterationRange: node.IterationRange,
	}, nil
}

// UnaryTestValue is the runtime value of a parenthesised unary test used
// as a value, e.g. `(< 10)`. It's only comparable to other UnaryTestValues
// with the same operator.
type UnaryTestValue struct {
	Op    string
	Value any
}

func (utv UnaryTestValue) String() string {
	return fmt.Sprintf("(%s%v)", utv.Op, utv.Value)
}

func (node UnaryTestValueNode) Eval(intp *Interpreter) (any, error) {
	val, err := node.Value.Eval(intp)
	if err != nil {
		return nil, err
	}
	return &UnaryTestValue{Op: node.Op, Value: val}, nil
}

func (node ArrayNode) Eval(intp *Interpreter) (any, error) {
	var arr []any
	for _, elem := range node.Elements {
		v, err := elem.Eval(intp)
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	return arr, nil
}
func (node EmptyNode) Eval(intp *Interpreter) (any, error) {
	return nil, nil
}

func (node MultiTests) Eval(intp *Interpreter) (any, error) {
	for _, elem := range node.Elements {
		v, err := elem.Eval(intp)
		if err != nil {
			return nil, err
		}
		if boolValue(v) {
			return true, nil
		}
	}
	return false, nil
}

func (node MapNode) Eval(intp *Interpreter) (any, error) {
	mapVal := make(map[string]any)
	intp.PushEmpty()
	defer intp.Pop()
	for _, item := range node.Values {
		if _, dup := mapVal[item.Name]; dup {
			return nil, fmt.Errorf("duplicate context entry key %q", item.Name)
		}
		v, err := item.Value.Eval(intp)
		if err != nil {
			return nil, err
		}
		mapVal[item.Name] = v
		intp.Bind(item.Name, v)
	}
	return mapVal, nil
}

func (node DotOp) Eval(intp *Interpreter) (any, error) {
	leftVal, err := node.Left.Eval(intp)
	if err != nil {
		return nil, err
	}
	if listVal, ok := leftVal.([]any); ok {
		result := make([]any, len(listVal))
		for i, elem := range listVal {
			if mapElem, ok := elem.(map[string]any); ok {
				if val, found := mapElem[node.Attr]; found {
					result[i] = val
					continue
				}
			}
			result[i] = Null
		}
		return result, nil
	} else if mapVal, ok := leftVal.(map[string]any); ok {
		if val, found := mapVal[node.Attr]; found {
			return val, nil
		} else {
			return nil, NewErrKeyNotFound(node.Attr)
		}
	} else if obj, ok := leftVal.(HasAttrs); ok {
		if v, found := obj.GetAttr(node.Attr); found {
			return normalizeValue(v), nil
		} else {
			//return nil, NewEvalError(-4001, "attr error", fmt.Sprintf("cannot get attr %s", node.Attr))
			return nil, NewErrKeyNotFound(node.Attr)

		}
	} else {
		return nil, NewErrTypeMismatch("map")
		//return Null, nil
	}
}

func (node IfExpr) Eval(intp *Interpreter) (any, error) {
	condVal, err := node.Cond.Eval(intp)
	if err != nil {
		return nil, err
	}

	if boolValue(condVal) {
		brVal, err := node.ThenBranch.Eval(intp)
		if err != nil {
			return nil, err
		}
		return brVal, nil
	} else {
		brVal, err := node.ElseBranch.Eval(intp)
		if err != nil {
			return nil, err
		}
		return brVal, nil
	}
}

func (node ForExpr) Eval(intp *Interpreter) (any, error) {
	// Collect a chain of comma-separated "for a in X, b in Y, ... return Z"
	// clauses (as opposed to a literal nested "for" written as the return
	// expression) so they can be evaluated as a single flattened cartesian
	// product rather than a nested list.
	type forClause struct {
		varname  string
		listExpr Node
	}
	clauses := []forClause{{node.Varname, node.ListExpr}}
	returnExpr := node.ReturnExpr
	for {
		next, ok := returnExpr.(*ForExpr)
		if !ok || !next.Chained {
			break
		}
		clauses = append(clauses, forClause{next.Varname, next.ListExpr})
		returnExpr = next.ReturnExpr
	}

	intp.PushEmpty()
	defer intp.Pop()

	results := make([]any, 0)
	var recurse func(i int) error
	recurse = func(i int) error {
		if i == len(clauses) {
			// "partial" is an implicit variable available inside the return
			// expression, bound to the list of results accumulated by
			// prior iterations (enabling recursive-style accumulation,
			// e.g. a factorial built via i * partial[-1]).
			intp.Bind("partial", results)
			res, err := returnExpr.Eval(intp)
			if err != nil {
				return err
			}
			results = append(results, res)
			return nil
		}
		listLike, err := clauses[i].listExpr.Eval(intp)
		if err != nil {
			return err
		}
		aList, err := materializeIterationList(listLike)
		if err != nil {
			return err
		}
		for _, val := range aList {
			intp.Bind(clauses[i].varname, val)
			if err := recurse(i + 1); err != nil {
				return err
			}
		}
		return nil
	}

	if err := recurse(0); err != nil {
		return nil, err
	}
	return results, nil
}

func (node SomeExpr) Eval(intp *Interpreter) (any, error) {
	listLike, err := node.ListExpr.Eval(intp)
	if err != nil {
		return nil, err
	}

	aList, err := materializeIterationList(listLike)
	if err != nil {
		return nil, err
	}
	intp.PushEmpty()
	for _, val := range aList {
		intp.Bind(node.Varname, val)

		res, err := node.FilterExpr.Eval(intp)
		if err != nil {
			intp.Pop()
			return nil, err
		}
		if boolValue(res) {
			return val, nil
		}
	}
	intp.Pop()
	return nil, nil
}

func (node EveryExpr) Eval(intp *Interpreter) (any, error) {
	listLike, err := node.ListExpr.Eval(intp)
	if err != nil {
		return nil, err
	}

	aList, err := materializeIterationList(listLike)
	if err != nil {
		return nil, err
	}
	intp.PushEmpty()
	chooses := make([]any, 0)
	for _, val := range aList {
		intp.Bind(node.Varname, val)

		res, err := node.FilterExpr.Eval(intp)
		if err != nil {
			intp.Pop()
			return nil, err
		}

		if boolValue(res) {
			chooses = append(chooses, val)
		}
	}
	intp.Pop()
	return chooses, nil
}

func (node FunDef) Eval(intp *Interpreter) (any, error) {
	return &FunDef{
		Args:    node.Args,
		Body:    node.Body,
		Closure: append([]Scope(nil), intp.ScopeStack...),
	}, nil
}

func (node FunDef) EvalCall(intp *Interpreter, args []any) (any, error) {
	if len(args) != len(node.Args) {
		return nil, errors.New("eval call argument size mismatch")
	}

	callIntp := intp
	if node.Closure != nil {
		callIntp = &Interpreter{ScopeStack: append([]Scope(nil), node.Closure...)}
	}

	callIntp.PushEmpty()
	defer callIntp.Pop()
	for i, argName := range node.Args {
		callIntp.Bind(argName, args[i])
	}
	return node.Body.Eval(callIntp)
}

func (node FunCall) Eval(intp *Interpreter) (any, error) {
	v, err := node.FunRef.Eval(intp)
	if err != nil {
		return nil, err
	}
	switch r := v.(type) {
	case *FunDef:
		return node.EvalFunDef(intp, r)
	case *NativeFun:
		return node.EvalNativeFun(intp, r)
	case *Macro:
		return node.EvalMacro(intp, r)
	case *RawFunc:
		return r.fn(intp, node)
	default:
		return Null, nil
	}
}

func (node FunCall) EvalNativeFun(intp *Interpreter, funDef *NativeFun) (any, error) {
	argVals := make(map[string]any)
	if node.keywordArgs {
		kwArgMap, err := node.evalArgsToMap(intp)
		if err != nil {
			return nil, err
		}
		for alias, canonical := range funDef.argAliases {
			if v, ok := kwArgMap[alias]; ok {
				if _, taken := kwArgMap[canonical]; !taken {
					kwArgMap[canonical] = v
				}
				delete(kwArgMap, alias)
			}
		}

		knownKwArgs := make(map[string]bool, len(funDef.requiredArgNames)+len(funDef.optionalArgNames))
		for _, argName := range funDef.requiredArgNames {
			knownKwArgs[argName] = true
			if v, ok := kwArgMap[argName]; ok {
				argVals[argName] = v
			} else {
				return Null, nil
			}
		}
		for _, argName := range funDef.optionalArgNames {
			knownKwArgs[argName] = true
			if v, ok := kwArgMap[argName]; ok {
				argVals[argName] = v
			}
		}
		if funDef.varArgName != "" {
			for k, v := range kwArgMap {
				if !knownKwArgs[k] {
					if k != funDef.varArgName {
						return Null, nil
					}
					if vars, ok := argVals[funDef.varArgName]; ok {
						varargs := vars.([]any)
						varargs = append(varargs, v)
						argVals[funDef.varArgName] = varargs
					} else {
						argVals[funDef.varArgName] = []any{v}
					}
				}
			}
		} else {
			for k := range kwArgMap {
				if !knownKwArgs[k] {
					return Null, nil
				}
			}
		}
	} else {
		if len(node.Args) < len(funDef.requiredArgNames) {
			return Null, nil
		}
		for i, argNode := range node.Args {
			a, err := argNode.arg.Eval(intp)
			if err != nil {
				return nil, err
			}
			if i < len(funDef.requiredArgNames) {
				argVals[funDef.requiredArgNames[i]] = a
			} else if i < len(funDef.requiredArgNames)+len(funDef.optionalArgNames) {
				argVals[funDef.optionalArgNames[i-len(funDef.requiredArgNames)]] = a
			} else if funDef.varArgName != "" {
				if vars, ok := argVals[funDef.varArgName]; ok {
					varargs := vars.([]any)
					varargs = append(varargs, a)
					argVals[funDef.varArgName] = varargs
				} else {
					argVals[funDef.varArgName] = []any{a}
				}
			} else {
				return Null, nil
			}
		}
	}
	return funDef.Call(intp, argVals)
}

func (node FunCall) evalArgsToMap(intp *Interpreter) (map[string]any, error) {
	if !node.keywordArgs {
		return nil, errors.New("funcall has no keyword args")
	}
	kwArgMap := make(map[string]any)
	for _, argNode := range node.Args {
		a, err := argNode.arg.Eval(intp)
		if err != nil {
			return nil, err
		}
		kwArgMap[argNode.argName] = a
	}
	return kwArgMap, nil
}

func (node FunCall) EvalMacro(intp *Interpreter, macro *Macro) (any, error) {
	if len(macro.requiredArgNames) > len(node.Args) {
		return nil, NewErrTooFewArguments(macro.requiredArgNames[len(node.Args):])
		//return nil, NewEvalError(-1005, "number of args of macro mismatch")
	}

	argNodes := make(map[string]Node)
	var varArgs []Node
	if node.keywordArgs {
		kwArgMap := make(map[string]Node)
		for _, argNode := range node.Args {
			kwArgMap[argNode.argName] = argNode.arg
		}

		for _, argName := range macro.requiredArgNames {
			if ast, ok := kwArgMap[argName]; ok {
				argNodes[argName] = ast
			} else {
				//return nil, NewEvalError(-5001, "no keyword argument", fmt.Sprintf("no keyword argument %s", argName))
				return nil, NewErrKeywordArgument(argName)
			}
		}

		for _, argName := range macro.optionalArgNames {
			if ast, ok := kwArgMap[argName]; ok {
				argNodes[argName] = ast
			}
		}
		if macro.varArgName != "" {
			knownKwArgs := make(map[string]bool, len(macro.requiredArgNames)+len(macro.optionalArgNames))
			for _, n := range macro.requiredArgNames {
				knownKwArgs[n] = true
			}
			for _, n := range macro.optionalArgNames {
				knownKwArgs[n] = true
			}
			for k, v := range kwArgMap {
				if !knownKwArgs[k] {
					varArgs = append(varArgs, v)
				}
			}
		}
	} else {
		if len(node.Args) < len(macro.requiredArgNames) {
			//reqArgs := strings.Join(macro.requiredArgNames[len(node.Args):len(macro.requiredArgNames)], ", ")
			//return nil, NewEvalError(-5003, "too few arguments", fmt.Sprintf("more arguments required: %s", reqArgs))
			return nil, NewErrTooFewArguments(macro.requiredArgNames[len(node.Args):])
		}
		for i, argNode := range node.Args {
			if i < len(macro.requiredArgNames) {
				argNodes[macro.requiredArgNames[i]] = argNode.arg
			} else if i < len(macro.requiredArgNames)+len(macro.optionalArgNames) {
				argNodes[macro.optionalArgNames[i-len(macro.requiredArgNames)]] = argNode.arg
			} else if macro.varArgName != "" {
				varArgs = append(varArgs, argNode.arg)
			} else {
				//return nil, NewEvalError(-5002, "too many arguments")
				return nil, NewErrTooManyArguments()
			}
		}
	}
	return macro.fn(intp, argNodes, varArgs)
}

func (node FunCall) EvalFunDef(intp *Interpreter, funDef *FunDef) (any, error) {
	if len(funDef.Args) > len(node.Args) {
		//return nil, NewEvalError(-1004, "number of args mismatch")
		return nil, NewErrTooFewArguments(funDef.Args[len(node.Args):])
	} else if len(funDef.Args) < len(node.Args) {
		return nil, NewErrTooManyArguments()
	}
	//var args []any
	// Arguments are evaluated against the caller's ambient scope (intp), but
	// the call is bound and the body evaluated against the function's own
	// lexical closure, if it captured one - otherwise a returned function
	// would lose access to its defining scope once back out in the caller.
	callIntp := intp
	if funDef.Closure != nil {
		callIntp = &Interpreter{ScopeStack: append([]Scope(nil), funDef.Closure...)}
	}
	callIntp.PushEmpty()
	defer callIntp.Pop()

	if node.keywordArgs {
		kwArgMap, err := node.evalArgsToMap(intp)
		if err != nil {
			return nil, err
		}

		for _, argName := range funDef.Args {
			if v, ok := kwArgMap[argName]; ok {
				//argVals = append(argVals, v)
				callIntp.Bind(argName, v)
			} else {
				//return nil, NewEvalError(-5001, "no keyword argument", fmt.Sprintf("no keyword argument %s", argName))
				callIntp.Bind(argName, Null)
			}
		}
	} else {
		for i, argNode := range node.Args {
			a, err := argNode.arg.Eval(intp)
			if err != nil {
				return nil, err
			}
			callIntp.Bind(funDef.Args[i], a)
		}
	}
	ret, err := funDef.Body.Eval(callIntp)
	return ret, err
}

func EvalString(input string, varsList ...string) (any, error) {
	intp := NewIntepreter()
	for i, vars := range varsList {
		if vars == "" {
			continue
		}
		scopeAst, err := ParseString(vars)
		if err != nil {
			return nil, err
		}
		r, err := scopeAst.Eval(intp)
		if err != nil {
			return nil, err
		}
		if scope, ok := r.(map[string]any); ok {
			intp.Push(scope)
		} else {
			return nil, fmt.Errorf("the NO. %d scope should be map", i+1)
		}
	}
	ast, err := ParseString(input)
	if err != nil {
		return nil, err
	}
	r, err := ast.Eval(intp)
	return r, err
}

func EvalStringWithScope(input string, scope Scope) (any, error) {
	ast, err := ParseString(input)
	if err != nil {
		return nil, err
	}
	intp := NewIntepreter()
	if scope != nil {
		intp.Push(scope)
	}
	r, err := ast.Eval(intp)
	return r, err
}
