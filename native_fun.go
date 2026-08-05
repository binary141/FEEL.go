package feel

import (
	"fmt"
	"sync"
)

// arg type error
type ArgTypeError struct {
	index  int
	expect string
}

func (err ArgTypeError) Error() string {
	return fmt.Sprintf("type error at %d, expect %s", err.index, err.expect)
}

// ArgSizeError
type ArgSizeError struct {
	has    int
	expect int
}

func (err ArgSizeError) Error() string {
	return fmt.Sprintf("argument size error, has %d, expect %d", err.has, err.expect)
}

// native function
type NativeFunDef func(args map[string]any) (any, error)

type NativeFun struct {
	fn               NativeFunDef
	requiredArgNames []string
	optionalArgNames []string
	varArgName       string
	help             string
	// argAliases maps an alternate keyword-argument name to the canonical
	// required/optional name it stands in for, for built-ins whose
	// parameter names have changed across FEEL spec versions (e.g. "m" for
	// "context").
	argAliases map[string]string
}

func NewNativeFunc(fn NativeFunDef) *NativeFun {
	return &NativeFun{fn: fn}
}

func (nfun *NativeFun) Required(argNames ...string) *NativeFun {
	nfun.requiredArgNames = append(nfun.requiredArgNames, argNames...)
	return nfun
}

func (nfun *NativeFun) Optional(argNames ...string) *NativeFun {
	nfun.optionalArgNames = append(nfun.optionalArgNames, argNames...)
	return nfun
}

func (nfun *NativeFun) Alias(canonical, alias string) *NativeFun {
	if nfun.argAliases == nil {
		nfun.argAliases = make(map[string]string)
	}
	nfun.argAliases[alias] = canonical
	return nfun
}

func (nfun *NativeFun) Vararg(argName string) *NativeFun {
	nfun.varArgName = argName
	return nfun
}

func (nfun *NativeFun) Help(help string) *NativeFun {
	nfun.help = help
	return nfun
}

func (nfun NativeFun) ArgNameAt(at int) (string, bool) {
	if at >= 0 && at < len(nfun.requiredArgNames) {
		return nfun.requiredArgNames[at], true
	} else if at >= len(nfun.requiredArgNames) && at < len(nfun.requiredArgNames)+len(nfun.optionalArgNames) {
		return nfun.optionalArgNames[at-len(nfun.requiredArgNames)], true
	}
	return "", false
}

func (nfun *NativeFun) Call(intp *Interpreter, args map[string]any) (any, error) {
	v, err := nfun.fn(args)
	if err != nil {
		return nil, err
	}
	return normalizeValue(v), nil
}

// RawFunc is a built-in with full control over argument dispatch: it sees
// the raw FunCall (positional or named, any arity) instead of having
// arguments pre-mapped to a fixed declared name list. Used for builtins
// whose accepted parameter names differ by arity (e.g. date(from) vs.
// date(year, month, day)).
type RawFuncDef func(intp *Interpreter, node FunCall) (any, error)

type RawFunc struct {
	fn RawFuncDef
}

func NewRawFunc(fn RawFuncDef) *RawFunc {
	return &RawFunc{fn: fn}
}

// macro
type MacroDef func(intp *Interpreter, args map[string]Node, varArgs []Node) (any, error)
type Macro struct {
	fn               MacroDef
	requiredArgNames []string
	optionalArgNames []string
	varArgName       string
	help             string
}

func NewMacro(fn MacroDef) *Macro {
	return &Macro{fn: fn}
}

func (macro *Macro) Required(argNames ...string) *Macro {
	macro.requiredArgNames = append(macro.requiredArgNames, argNames...)
	return macro
}

func (macro *Macro) Optional(argNames ...string) *Macro {
	macro.optionalArgNames = append(macro.optionalArgNames, argNames...)
	return macro
}
func (macro *Macro) Vararg(argName string) *Macro {
	macro.varArgName = argName
	return macro
}
func (macro *Macro) Help(help string) *Macro {
	macro.help = help
	return macro
}

// Prelude
type Prelude struct {
	vars map[string]any
}

var loadOnce sync.Once
var inst *Prelude

func GetPrelude() *Prelude {
	loadOnce.Do(func() {
		inst = &Prelude{vars: make(map[string]any)}
		inst.Load()
	})
	return inst
}

func (prelude *Prelude) Load() {
	// prelude.Bind("bind", NewMacro(func(intp *Interpreter, args map[string]Node, varArgs []Node) (any, error) {
	// 	name, _ := args["name"].Eval(intp)
	// 	strName, ok := name.(string)
	// 	if !ok {
	// 		return nil, NewErrTypeMismatch("string")
	// 	}
	// 	v, err := args["value"].Eval(intp)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	intp.Bind(strName, v)
	// 	return v, nil
	// }).Required("name", "value").Help("bind value to name in current top scope"))

	// prelude.Bind("set", NewMacro(func(intp *Interpreter, args map[string]Node, varArgs []Node) (any, error) {
	// 	name, _ := args["name"].Eval(intp)
	// 	strName, ok := name.(string)
	// 	if !ok {
	// 		return nil, NewErrTypeMismatch("string")
	// 	}
	// 	v, err := args["value"].Eval(intp)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	if intp.Set(strName, v) {
	// 		return v, nil
	// 	} else {
	// 		intp.Bind(strName, v)
	// 		return v, nil
	// 	}
	// }).Required("name", "value").Help("bind value to name in resolved scope, if not found, it's bind to current top scope(the same as 'bind')"))

	prelude.Bind("block", NewMacro(func(intp *Interpreter, args map[string]Node, exprlist []Node) (any, error) {
		var lastValue any
		var err error
		for _, expr := range exprlist {
			lastValue, err = expr.Eval(intp)
			if err != nil {
				return nil, err
			}
		}
		return lastValue, nil
	}).Vararg("express list").Help("quote a sequence of expresses and return the last result"))

	prelude.Bind("help", NewMacro(func(intp *Interpreter, args map[string]Node, exprlist []Node) (any, error) {
		v, err := args["value"].Eval(intp)
		if err != nil {
			return nil, err
		}
		switch vv := v.(type) {
		case *NativeFun:
			return vv.help, nil
		case *Macro:
			return vv.help, nil
		default:
			return typeName(vv), nil
		}
	}).Required("value").Help("the help information of a value"))

	prelude.Bind("typeof", NewMacro(func(intp *Interpreter, args map[string]Node, exprlist []Node) (any, error) {
		v, err := args["value"].Eval(intp)
		if err != nil {
			return nil, err
		}
		return typeName(v), nil
	}).Required("value").Help("the type of a value"))

	installDatetimeFunctions(prelude)
	installBuiltinFunctions(prelude)
	installContextFunctions(prelude)
	installRangeFunctions(prelude)
}

func (prelude *Prelude) Bind(name string, value any) *Prelude {
	if _, ok := prelude.vars[name]; ok {
		panic(fmt.Sprintf("bind(), name '%s' already bound", name))
	}
	prelude.vars[name] = normalizeValue(value)
	return prelude
}

func (prelude *Prelude) Resolve(name string) (any, bool) {
	v, ok := prelude.vars[name]
	return v, ok
}

// buildin native funcs
func nativeBind(intp *Interpreter, varname string, value any) (any, error) {
	intp.Bind(varname, value)
	return nil, nil
}

// callableArity returns the number of positional parameters v (a *FunDef or
// *NativeFun) accepts, for builtins (sort, list replace, ...) that take a
// function value as ordinary data and need to validate its arity before
// calling it positionally.
func callableArity(v any) (int, bool) {
	switch fn := v.(type) {
	case *FunDef:
		return len(fn.Args), true
	case *NativeFun:
		return len(fn.requiredArgNames) + len(fn.optionalArgNames), true
	default:
		return 0, false
	}
}

// callPositional invokes v (a *FunDef or *NativeFun) with positional
// arguments, for builtins that receive a function value as data rather than
// calling it through normal FunCall syntax.
func callPositional(intp *Interpreter, v any, args []any) (any, bool, error) {
	switch fn := v.(type) {
	case *FunDef:
		r, err := fn.EvalCall(intp, args)
		return r, true, err
	case *NativeFun:
		argMap := make(map[string]any, len(args))
		for i, a := range args {
			name, ok := fn.ArgNameAt(i)
			if !ok {
				break
			}
			argMap[name] = a
		}
		r, err := fn.Call(intp, argMap)
		return r, true, err
	default:
		return nil, false, nil
	}
}
