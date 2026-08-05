package feel

import (
	"fmt"
	"strconv"
	"strings"
)

type Scope map[string]any

type Interpreter struct {
	ScopeStack []Scope

	// TypeResolver, when set, resolves a custom type name (e.g. a DMN
	// itemDefinition alias like "tEligibility") used on the right of
	// "instance of" into the structural type descriptor "instance of"
	// itself understands (a builtin primitive name, or a "list<...>" /
	// "context<...>" descriptor). Names it doesn't recognize should return
	// ok=false so the type name is checked as-is.
	TypeResolver func(name string) (string, bool)
}

type Node interface {
	Repr() string
	Eval(*Interpreter) (any, error)
	TextRange() TextRange
}

type HasAttrs interface {
	GetAttr(name string) (any, bool)
}

type TextRange struct {
	Start ScanPosition
	End   ScanPosition
}

// binary operator
type Binop struct {
	Op    string
	Left  Node
	Right Node

	textRange TextRange
}

func (op Binop) TextRange() TextRange {
	return op.textRange
}
func (op Binop) Repr() string {
	return fmt.Sprintf("(%s %s %s)", op.Op, op.Left.Repr(), op.Right.Repr())
}

// function call
type DotOp struct {
	Left Node
	Attr string

	textRange TextRange
}

func (op DotOp) TextRange() TextRange {
	return op.textRange
}

func (op DotOp) Repr() string {
	return fmt.Sprintf("(. %s %s)", op.Left.Repr(), op.Attr)
}

// function call
type funcallArg struct {
	argName string
	arg     Node
}

type FunCall struct {
	FunRef      Node
	Args        []funcallArg
	keywordArgs bool

	textRange TextRange
}

func (fc FunCall) TextRange() TextRange {
	return fc.textRange
}
func (fc FunCall) Repr() string {
	argReprs := make([]string, 0)
	if fc.keywordArgs {
		for _, arg := range fc.Args {
			s := fmt.Sprintf("(%s %s)", arg.argName, arg.arg.Repr())
			argReprs = append(argReprs, s)
		}
	} else {
		for _, arg := range fc.Args {
			argReprs = append(argReprs, arg.arg.Repr())
		}
	}
	return fmt.Sprintf("(call %s [%s])", fc.FunRef.Repr(), strings.Join(argReprs, ", "))
}

// function definition
type FunDef struct {
	Args []string
	Body Node

	// Closure is the lexical scope chain captured when this function value
	// was created (see FunDef.Eval), so calling it later - e.g. after it has
	// been returned out of its defining function - still sees the variables
	// visible at its definition site rather than whatever happens to be on
	// the caller's scope stack.
	Closure []Scope

	textRange TextRange
}

func (fdef FunDef) TextRange() TextRange {
	return fdef.textRange
}

func (fdef FunDef) Repr() string {
	return fmt.Sprintf("(function [%s] %s)", strings.Join(fdef.Args, ", "), fdef.Body.Repr())
}

// variable
type Var struct {
	Name      string
	textRange TextRange
}

func (v Var) TextRange() TextRange {
	return v.textRange
}

func (v Var) Repr() string {
	if strings.Contains(v.Name, " ") {
		return fmt.Sprintf("`%s`", v.Name)
	}
	return v.Name
}

// number
// valueNode wraps an already-evaluated value as a Node, so it can be
// reused as an operand to existing AST evaluation logic (e.g. NegateNode
// falling back to subtraction-from-zero for operand types with no direct
// negation, without re-evaluating the original operand expression).
type valueNode struct {
	value     any
	textRange TextRange
}

func (node valueNode) TextRange() TextRange {
	return node.textRange
}
func (node valueNode) Repr() string {
	return fmt.Sprintf("%v", node.value)
}
func (node valueNode) Eval(intp *Interpreter) (any, error) {
	return node.value, nil
}

// NegateNode is FEEL unary minus ("-expr"). Numbers and durations negate
// directly; other operand types (e.g. date/time) fall back to the
// "0 - operand" arithmetic used for numeric negation, which correctly
// surfaces as a type-mismatch error for operands that can't be negated.
type NegateNode struct {
	Operand Node

	textRange TextRange
}

func (node NegateNode) TextRange() TextRange {
	return node.textRange
}
func (node NegateNode) Repr() string {
	return fmt.Sprintf("(neg %s)", node.Operand.Repr())
}

type NumberNode struct {
	Value string

	textRange TextRange
}

func (node NumberNode) TextRange() TextRange {
	return node.textRange
}

func (node NumberNode) Repr() string {
	return node.Value
}

// bool
type BoolNode struct {
	Value bool

	textRange TextRange
}

func (node BoolNode) TextRange() TextRange {
	return node.textRange
}
func (node BoolNode) Repr() string {
	if node.Value {
		return "true"
	} else {
		return "false"
	}
}

// null
type NullNode struct {
	textRange TextRange
}

func (node NullNode) Repr() string {
	return "null"
}

func (node NullNode) TextRange() TextRange {
	return node.textRange
}

// string
type StringNode struct {
	Value string

	textRange TextRange
}

func (node StringNode) Repr() string {
	return node.Value
}
func (node StringNode) TextRange() TextRange {
	return node.textRange
}
func (node StringNode) Content() string {
	// trim leading and trailing quotes
	s := node.Value[1 : len(node.Value)-1]
	return unescapeFeelString(s)
}

// unescapeFeelString processes FEEL string escape sequences: \n \r \t \" \'
// \\, the 4-hex-digit \uXXXX form (including UTF-16 surrogate pairs for
// codepoints beyond the BMP), and the 6-hex-digit \UXXXXXX form for a full
// unicode codepoint.
func unescapeFeelString(s string) string {
	runes := []rune(s)
	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for i < len(runes) {
		c := runes[i]
		if c == '\\' && i+1 < len(runes) {
			switch runes[i+1] {
			case 'n':
				sb.WriteByte('\n')
				i += 2
				continue
			case 'r':
				sb.WriteByte('\r')
				i += 2
				continue
			case 't':
				sb.WriteByte('\t')
				i += 2
				continue
			case '"':
				sb.WriteByte('"')
				i += 2
				continue
			case '\'':
				sb.WriteByte('\'')
				i += 2
				continue
			case '\\':
				sb.WriteByte('\\')
				i += 2
				continue
			case 'u':
				if i+6 <= len(runes) {
					if code, err := strconv.ParseUint(string(runes[i+2:i+6]), 16, 32); err == nil {
						r1 := rune(code)
						if r1 >= 0xD800 && r1 <= 0xDBFF && i+12 <= len(runes) &&
							runes[i+6] == '\\' && runes[i+7] == 'u' {
							if code2, err2 := strconv.ParseUint(string(runes[i+8:i+12]), 16, 32); err2 == nil {
								r2 := rune(code2)
								if r2 >= 0xDC00 && r2 <= 0xDFFF {
									combined := ((r1 - 0xD800) << 10) + (r2 - 0xDC00) + 0x10000
									sb.WriteRune(combined)
									i += 12
									continue
								}
							}
						}
						sb.WriteRune(r1)
						i += 6
						continue
					}
				}
			case 'U':
				if i+8 <= len(runes) {
					if code, err := strconv.ParseUint(string(runes[i+2:i+8]), 16, 32); err == nil {
						sb.WriteRune(rune(code))
						i += 8
						continue
					}
				}
			}
		}
		sb.WriteRune(c)
		i++
	}
	return sb.String()
}

// Map

type mapItem struct {
	Name  string
	Value Node
}

type MapNode struct {
	Values []mapItem

	textRange TextRange
}

func (node MapNode) TextRange() TextRange {
	return node.textRange
}
func (node MapNode) Repr() string {
	var ss []string
	for _, item := range node.Values {
		s := fmt.Sprintf("(\"%s\" %s)", item.Name, item.Value.Repr())
		ss = append(ss, s)
	}
	return fmt.Sprintf("(map %s)", strings.Join(ss, " "))
}

// temporal
type TemporalNode struct {
	Value     string
	textRange TextRange
}

func (node TemporalNode) TextRange() TextRange {
	return node.textRange
}
func (node TemporalNode) Repr() string {
	return node.Value
}

func (node TemporalNode) Content() string {
	return node.Value[2 : len(node.Value)-1]
}

// range
type RangeNode struct {
	StartOpen bool
	Start     Node

	EndOpen bool
	End     Node

	// IterationRange marks a bare "start..end" range written directly as a
	// "for"/"some"/"every" iteration source (as opposed to an explicit
	// range literal like "[start..end]"). Only iteration ranges may
	// descend (start > end); an explicit range literal with a descending
	// endpoint pair is invalid.
	IterationRange bool

	textRange TextRange
}

func (node RangeNode) TextRange() TextRange {
	return node.textRange
}
func (node RangeNode) Repr() string {
	startQuote := "["
	if node.StartOpen {
		startQuote = "("
	}
	endQuote := "]"
	if node.EndOpen {
		endQuote = ")"
	}
	return fmt.Sprintf("%s%s..%s%s", startQuote, node.Start.Repr(), node.End.Repr(), endQuote)
}

// UnaryTestValueNode represents a parenthesised unary test used as a
// value, e.g. `(< 10)` or `(=10)`. These aren't evaluated against an
// implicit `?`; they evaluate to a comparable UnaryTestValue so that
// expressions like `(< 10) = (< 10)` can be checked for equality.
type UnaryTestValueNode struct {
	Op    string
	Value Node

	textRange TextRange
}

func (node UnaryTestValueNode) TextRange() TextRange {
	return node.textRange
}
func (node UnaryTestValueNode) Repr() string {
	return fmt.Sprintf("(%s%s)", node.Op, node.Value.Repr())
}

// if expression
type IfExpr struct {
	Cond       Node
	ThenBranch Node
	ElseBranch Node

	textRange TextRange
}

func (node IfExpr) TextRange() TextRange {
	return node.textRange
}
func (node IfExpr) Repr() string {
	return fmt.Sprintf("(if %s %s %s)", node.Cond.Repr(), node.ThenBranch.Repr(), node.ElseBranch.Repr())
}

// array
type ArrayNode struct {
	Elements []Node

	textRange TextRange
}

func (node ArrayNode) TextRange() TextRange {
	return node.textRange
}
func (node ArrayNode) Repr() string {
	s := make([]string, 0)
	for _, elem := range node.Elements {
		s = append(s, elem.Repr())
	}
	return fmt.Sprintf("[%s]", strings.Join(s, ", "))
}

// Empty node
type EmptyNode struct {
	textRange TextRange
}

func (node EmptyNode) TextRange() TextRange {
	return node.textRange
}
func (node EmptyNode) Repr() string {
	return ""
}

// MultiTests
type MultiTests struct {
	Elements  []Node
	textRange TextRange
}

func (node MultiTests) TextRange() TextRange {
	return node.textRange
}
func (node MultiTests) Repr() string {
	s := make([]string, 0)
	for _, elem := range node.Elements {
		s = append(s, elem.Repr())
	}
	return fmt.Sprintf("(multitests %s)", strings.Join(s, " "))
}

// between expression: value between lower and upper
type BetweenExpr struct {
	Value     Node
	Lower     Node
	Upper     Node
	textRange TextRange
}

func (node BetweenExpr) TextRange() TextRange {
	return node.textRange
}
func (node BetweenExpr) Repr() string {
	return fmt.Sprintf("(between %s %s %s)", node.Value.Repr(), node.Lower.Repr(), node.Upper.Repr())
}

// For expression
type ForExpr struct {
	Varname    string
	ListExpr   Node
	ReturnExpr Node
	// Chained is true when this node was produced by a comma-separated
	// "for a in X, b in Y return Z" clause, as opposed to a literal nested
	// "for" expression written as the return expression. Chained clauses
	// form a cartesian product flattened into a single result list; a
	// genuinely nested "for" produces a nested list.
	Chained   bool
	textRange TextRange
}

func (node ForExpr) TextRange() TextRange {
	return node.textRange
}
func (node ForExpr) Repr() string {
	return fmt.Sprintf("(for %s %s %s)", node.Varname, node.ListExpr.Repr(), node.ReturnExpr.Repr())
}

// Some expression
type SomeExpr struct {
	Varname    string
	ListExpr   Node
	FilterExpr Node
	textRange  TextRange
}

func (node SomeExpr) TextRange() TextRange {
	return node.textRange
}
func (node SomeExpr) Repr() string {
	return fmt.Sprintf("(some \"%s\" %s %s)", node.Varname, node.ListExpr.Repr(), node.FilterExpr.Repr())
}

// instance of expression: value instance of typeName
type InstanceOfNode struct {
	Value     Node
	TypeName  string
	textRange TextRange
}

func (node InstanceOfNode) TextRange() TextRange {
	return node.textRange
}
func (node InstanceOfNode) Repr() string {
	return fmt.Sprintf("(instance-of %s %s)", node.Value.Repr(), node.TypeName)
}

// Every expression
type EveryExpr struct {
	Varname    string
	ListExpr   Node
	FilterExpr Node

	textRange TextRange
}

func (node EveryExpr) TextRange() TextRange {
	return node.textRange
}
func (node EveryExpr) Repr() string {
	return fmt.Sprintf("(every \"%s\" %s %s)", node.Varname, node.ListExpr.Repr(), node.FilterExpr.Repr())
}
