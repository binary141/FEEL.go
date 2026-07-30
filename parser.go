package feel

// for FEEL syntax refer to https://learn-dmn-in-15-minutes.com/learn/the-feel-language.html
// for BNF forms and handbook refer to https://kiegroup.github.io/dmn-feel-handbook

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
)

type UnexpectedToken struct {
	token   ScannerToken
	callers []string
	expects []string
}

func NewUnexpectedToken(token ScannerToken, callers []string, expects []string) *UnexpectedToken {
	return &UnexpectedToken{token: token, callers: callers, expects: expects}
}

func (err UnexpectedToken) Error() string {
	return fmt.Sprintf(
		"unexpected %s %s, at %d %d, expect %s\ncallers:\n%s\n",
		err.token.Kind, err.token.Value,
		err.token.Pos.Row, err.token.Pos.Column,
		strings.Join(err.expects, ", "),
		strings.Join(err.callers, "\n"),
	)
}

func hasDupName(names []string) (bool, string) {
	nameSet := make(map[string]bool)
	for _, name := range names {
		if _, ok := nameSet[name]; ok {
			return true, name
		}
		nameSet[name] = true
	}
	return false, ""
}

func ParseString(input string) (Node, error) {
	parser := NewParser(NewScanner(input))
	return parser.Parse()
}

type Parser struct {
	scanner    *Scanner
	inRangeEnd bool // when true, '[' is not consumed as index in parseFuncallOrIndexOrDot
}

func NewParser(scanner *Scanner) *Parser {
	return &Parser{
		scanner: scanner,
	}
}

func (p Parser) Unexpected(expects ...string) *UnexpectedToken {
	// extract caller stack dump
	pc := make([]uintptr, 10)
	n := runtime.Callers(2, pc)
	var callers []string
	if n > 0 {
		pc = pc[:n]
		frames := runtime.CallersFrames(pc)
		for {
			frame, more := frames.Next()
			callers = append(callers, fmt.Sprintf("%s:%d", frame.Function, frame.Line))
			if !more {
				break
			}
		}
	}
	return NewUnexpectedToken(p.CurrentToken(), callers, expects)
}

func (p Parser) CurrentToken() ScannerToken {
	return p.scanner.Current()
}

func (p *Parser) Parse() (Node, error) {
	p.scanner.Next()
	if p.CurrentToken().Expect(TokenEOF) {
		return &EmptyNode{}, nil
	}
	exp, err := p.parseUnaryTests()
	if err != nil {
		return nil, err
	}
	return exp, err
}

func (p Parser) startTextRange() TextRange {
	return TextRange{Start: p.CurrentToken().Pos}
}

func (p *Parser) parseUnaryTest() (Node, error) {
	if p.CurrentToken().Expect(">", ">=", "<", "<=", "!=", "=") {
		textRange := p.startTextRange()
		op := p.CurrentToken().Kind
		p.scanner.Next()
		// Use expression() so the RHS can be a function call, boolean, etc.
		right, err := p.betweenOp()
		if err != nil {
			return nil, err
		}
		textRange.End = p.CurrentToken().Pos
		exp := &Binop{
			Left:      &Var{Name: "?"},
			Op:        op,
			Right:     right,
			textRange: textRange,
		}
		return exp, nil
	} else {
		return p.expression()
	}
}

func (p *Parser) parseUnaryTests() (Node, error) {
	textRange := p.startTextRange()
	exp, err := p.parseUnaryTest()
	if err != nil {
		return nil, err
	}

	if p.CurrentToken().Expect(",") {
		elements := []Node{exp}
		for p.CurrentToken().Expect(",") {
			p.scanner.Next()

			uexp, err := p.parseUnaryTest()
			if err != nil {
				return nil, err
			}
			elements = append(elements, uexp)
		}
		textRange.End = p.CurrentToken().Pos
		return &MultiTests{Elements: elements, textRange: textRange}, nil
	} else {
		return exp, nil
	}
}

func (p *Parser) expression() (Node, error) {
	return p.inOp()
}

type astFunc func() (Node, error)

func (p *Parser) binop(ops []string, subfunc astFunc) (Node, error) {
	left, err := subfunc()
	if err != nil {
		return nil, err
	}

	for p.CurrentToken().Expect(ops...) {
		op := p.CurrentToken().Kind
		p.scanner.Next()

		right, err := subfunc()
		if err != nil {
			return nil, err
		}
		textRange := TextRange{Start: left.TextRange().Start}
		textRange.End = p.CurrentToken().Pos
		left = &Binop{Op: op, Left: left, Right: right, textRange: textRange}
	}
	return left, nil
}

func (p *Parser) binopKeywords(ops []string, subfunc astFunc) (Node, error) {
	left, err := subfunc()
	if err != nil {
		return nil, err
	}

	for p.CurrentToken().ExpectKeywords(ops...) {
		op := p.CurrentToken().Value
		p.scanner.Next()

		right, err := subfunc()
		if err != nil {
			return nil, err
		}
		textRange := TextRange{Start: left.TextRange().Start}
		textRange.End = p.CurrentToken().Pos

		left = &Binop{Op: op, Left: left, Right: right, textRange: textRange}
	}
	return left, nil
}

// inOp parses "expr in <test>" where test can be a unary comparison,
// a parenthesised list of tests, or any expression (range, list, value).
func (p *Parser) inOp() (Node, error) {
	left, err := p.betweenOp()
	if err != nil {
		return nil, err
	}
	if !p.CurrentToken().ExpectKeywords("in") {
		return left, nil
	}
	p.scanner.Next()
	right, err := p.parseInRHS()
	if err != nil {
		return nil, err
	}
	textRange := TextRange{Start: left.TextRange().Start, End: p.CurrentToken().Pos}
	return &Binop{Op: "in", Left: left, Right: right, textRange: textRange}, nil
}

// parseInRHS parses the right-hand side of an `in` expression.
func (p *Parser) parseInRHS() (Node, error) {
	// Single unary comparison: < val, <= val, > val, >= val, = val, != val
	if p.CurrentToken().Expect(">", ">=", "<", "<=", "!=", "=") {
		return p.parseUnaryTest()
	}
	// Parenthesised: open-start range (a..b) or list of tests (a, <b, >=c)
	if p.CurrentToken().Expect("(") {
		return p.parseInParenRHS()
	}
	// Default: range, list literal, value expression
	return p.expression()
}

// parseInParenRHS handles the parenthesised form on the right of `in`.
func (p *Parser) parseInParenRHS() (Node, error) {
	textRange := p.startTextRange()
	p.scanner.Next() // consume '('

	first, err := p.parseUnaryTest()
	if err != nil {
		return nil, err
	}

	if p.CurrentToken().Expect("..") {
		// Open-start range: (first..end) or (first..end]
		p.scanner.Next()
		end, err := p.expression()
		if err != nil {
			return nil, err
		}
		endOpen := p.CurrentToken().Expect(")")
		if !p.CurrentToken().Expect(")", "]") {
			return nil, p.Unexpected(")", "]")
		}
		p.scanner.Next()
		textRange.End = p.CurrentToken().Pos
		return &RangeNode{StartOpen: true, Start: first, EndOpen: endOpen, End: end, textRange: textRange}, nil
	}

	if p.CurrentToken().Expect(",") {
		// List of tests: plain values become equality tests so (1, <5) means ?=1 or ?<5
		elements := []Node{wrapAsUnaryEq(first)}
		for p.CurrentToken().Expect(",") {
			p.scanner.Next()
			elem, err := p.parseUnaryTest()
			if err != nil {
				return nil, err
			}
			elements = append(elements, wrapAsUnaryEq(elem))
		}
		if !p.CurrentToken().Expect(")") {
			return nil, p.Unexpected(")")
		}
		p.scanner.Next()
		textRange.End = p.CurrentToken().Pos
		return &MultiTests{Elements: elements, textRange: textRange}, nil
	}

	// Single value in parens: (first)
	if !p.CurrentToken().Expect(")") {
		return nil, p.Unexpected(")")
	}
	p.scanner.Next()
	return first, nil
}

// wrapAsUnaryEq wraps a node as ?=node unless it is already a unary test
// (a Binop whose LHS is the implicit-input variable ?).
func wrapAsUnaryEq(n Node) Node {
	if b, ok := n.(*Binop); ok {
		if v, ok := b.Left.(*Var); ok && v.Name == "?" {
			return n
		}
	}
	return &Binop{Left: &Var{Name: "?"}, Op: "=", Right: n}
}

// parseInstanceOfTypeExpr parses the type name on the right of "instance
// of". It handles simple names ("number"), compound names ("date and
// time"), and parameterized types ("range<number>", "list<Any>",
// "context<a: string, b: number>", "function<string>-> number"). The
// returned string is a best-effort textual descriptor: FEEL.go doesn't
// implement full DMN structural typing, so InstanceOfNode.Eval only uses
// the outer type keyword for list/context/function and ignores the inner
// structure.
func (p *Parser) parseInstanceOfTypeExpr() (string, error) {
	if p.CurrentToken().Kind != TokenName &&
		!(p.CurrentToken().Kind == TokenKeyword && p.CurrentToken().Value == "function") {
		return "", p.Unexpected("type name")
	}
	typeName := p.CurrentToken().Value
	p.scanner.Next()

	// Handle compound type names ("date and time", "years and months
	// duration", "days and time duration").
	for p.CurrentToken().ExpectKeywords("and") {
		p.scanner.Next() // consume "and"
		if p.CurrentToken().Kind != TokenName {
			break
		}
		typeName += " and " + p.CurrentToken().Value
		p.scanner.Next() // consume name after "and"
		// "years and months duration" / "days and time duration" have a
		// trailing bare word after the "and" part.
		if p.CurrentToken().Kind == TokenName && (p.CurrentToken().Value == "duration") {
			typeName += " " + p.CurrentToken().Value
			p.scanner.Next()
		}
	}

	// Handle parameterized types (e.g. "range<number>", "list<Any>",
	// "context<a: string, b: number>", "function<string, number>").
	if p.CurrentToken().Expect("<") {
		p.scanner.Next() // consume '<'
		var parts []string
		for !p.CurrentToken().Expect(">") {
			part, err := p.parseInstanceOfTypeExpr()
			if err != nil {
				return "", err
			}
			if p.CurrentToken().Expect(":") {
				p.scanner.Next()
				valType, err := p.parseInstanceOfTypeExpr()
				if err != nil {
					return "", err
				}
				part = part + ": " + valType
			}
			parts = append(parts, part)
			if p.CurrentToken().Expect(",") {
				p.scanner.Next()
			} else {
				break
			}
		}
		if !p.CurrentToken().Expect(">") {
			return "", p.Unexpected(">")
		}
		p.scanner.Next() // consume '>'
		typeName = typeName + "<" + strings.Join(parts, ", ") + ">"
	}

	// Handle a function's "-> returnType" suffix.
	if typeName == "function" || strings.HasPrefix(typeName, "function<") {
		if p.CurrentToken().Expect("-") {
			p.scanner.Next()
			if !p.CurrentToken().Expect(">") {
				return "", p.Unexpected(">")
			}
			p.scanner.Next()
			retType, err := p.parseInstanceOfTypeExpr()
			if err != nil {
				return "", err
			}
			typeName = typeName + " -> " + retType
		}
	}

	return typeName, nil
}

func (p *Parser) betweenOp() (Node, error) {
	textRange := p.startTextRange()
	left, err := p.logicOrOp()
	if err != nil {
		return nil, err
	}
	if p.CurrentToken().Kind == TokenName && p.CurrentToken().Value == "instance" {
		p.scanner.Next()
		if !(p.CurrentToken().Kind == TokenName && p.CurrentToken().Value == "of") {
			return nil, p.Unexpected("of")
		}
		p.scanner.Next()
		typeName, err := p.parseInstanceOfTypeExpr()
		if err != nil {
			return nil, err
		}
		textRange.End = p.CurrentToken().Pos
		return &InstanceOfNode{Value: left, TypeName: typeName, textRange: textRange}, nil
	}
	if !(p.CurrentToken().Kind == TokenName && p.CurrentToken().Value == "between") {
		return left, nil
	}
	p.scanner.Next()
	lower, err := p.compareOp()
	if err != nil {
		return nil, err
	}
	if !p.CurrentToken().ExpectKeywords("and") {
		return nil, p.Unexpected("and")
	}
	p.scanner.Next()
	upper, err := p.compareOp()
	if err != nil {
		return nil, err
	}
	textRange.End = p.CurrentToken().Pos
	return &BetweenExpr{Value: left, Lower: lower, Upper: upper, textRange: textRange}, nil
}

func (p *Parser) logicOrOp() (Node, error) {
	return p.binopKeywords(
		[]string{"or"},
		p.logicAndOp,
	)
}

func (p *Parser) logicAndOp() (Node, error) {
	return p.binopKeywords(
		[]string{"and"},
		p.compareOp,
	)
}

func (p *Parser) compareOp() (Node, error) {
	return p.binop(
		[]string{">", ">=", "<", "<=", "!=", "="},
		p.addOrSubOp,
	)
}

func (p *Parser) addOrSubOp() (Node, error) {
	return p.binop(
		[]string{"+", "-"},
		p.mulOrDivOp,
	)
}

func (p *Parser) mulOrDivOp() (Node, error) {
	return p.binop(
		[]string{"*", "/", "%"},
		p.powOp,
	)
}

func (p *Parser) powOp() (Node, error) {
	return p.binop(
		[]string{"**"},
		p.unaryMinusOp,
	)
}

func (p *Parser) unaryMinusOp() (Node, error) {
	if p.CurrentToken().Expect("-") {
		textRange := p.startTextRange()
		p.scanner.Next()
		right, err := p.unaryMinusOp()
		if err != nil {
			return nil, err
		}
		textRange.End = p.CurrentToken().Pos
		return &NegateNode{Operand: right, textRange: textRange}, nil
	}
	return p.parseFuncallOrIndexOrDot()
}

func (p *Parser) parseFuncallOrIndexOrDot() (Node, error) {
	exp, err := p.singleElement()
	if err != nil {
		return nil, err
	}
	for {
		switch p.CurrentToken().Kind {
		case "(":
			nexp, err := p.parseFuncallRest(exp)
			if err != nil {
				return nil, err
			}
			exp = nexp
		case "[":
			if p.inRangeEnd {
				return exp, nil
			}
			nexp, err := p.parseIndexRest(exp)
			if err != nil {
				return nil, err
			}
			exp = nexp
		case ".":
			nexp, err := p.parseDotRest(exp)
			if err != nil {
				return nil, err
			}
			exp = nexp
		default:
			return exp, nil
		}
	}
}

// var funcallTrailing = regexp.MustCompile(`\s*\($`)
// func (p *Parser) parseFuncall() (Node, error) {
// 	funcallWithRbracket := p.CurrentToken().Value
// 	funcName := funcallTrailing.ReplaceAllString(funcallWithRbracket, "")
// 	textRange := TextRange{Start: Node.TextRange().Start, End: p.CurrentToken().Pos}
// 	return p.parseFuncallRest(&Var{Name: funcName, textRange: })
// // }

func (p *Parser) parseFunccallArg() (funcallArg, error) {
	arg, err := p.expression()
	if err != nil {
		return funcallArg{}, err
	}

	if p.CurrentToken().Expect(":") { // kwargs
		if varArg, ok := arg.(*Var); ok {
			p.scanner.Next()
			argValue, err := p.expression()
			if err != nil {
				return funcallArg{}, err
			}
			return funcallArg{argName: varArg.Name, arg: argValue}, nil
		} else {
			return funcallArg{}, p.Unexpected("var")
		}
	} else {
		return funcallArg{argName: "", arg: arg}, nil
	}
}

func (p *Parser) parseFuncallRest(funExpr Node) (Node, error) {
	p.scanner.Next()
	// parse function arguments
	var args []funcallArg = nil
	keywordArgs := false
	for !p.CurrentToken().Expect(")") {
		arg, err := p.parseFunccallArg()
		if err != nil {
			return nil, err
		}
		if !keywordArgs && arg.argName != "" {
			keywordArgs = true
		}
		if len(args) > 0 {
			if arg.argName != "" && args[0].argName == "" {
				return nil, p.Unexpected("non var")
			}
			if arg.argName == "" && args[0].argName != "" {
				return nil, p.Unexpected("var")
			}
		}
		args = append(args, arg)
		if p.CurrentToken().Expect(",") {
			p.scanner.Next()
		} else if !p.CurrentToken().Expect(")") {
			return nil, p.Unexpected(",", ")")
		}
	}

	if p.CurrentToken().Expect(")") {
		p.scanner.Next()
	}

	textRange := TextRange{Start: funExpr.TextRange().Start, End: p.CurrentToken().Pos}
	return &FunCall{
		FunRef:      funExpr,
		Args:        args,
		keywordArgs: keywordArgs,
		textRange:   textRange,
	}, nil
}

func (p *Parser) parseIndexRest(exp Node) (Node, error) {
	p.scanner.Next()

	// parse index arguments
	at, err := p.expression()
	if err != nil {
		return nil, err
	}
	if !p.CurrentToken().Expect("]") {
		return nil, p.Unexpected("]")
	}

	p.scanner.Next()
	textRange := TextRange{Start: exp.TextRange().Start, End: p.CurrentToken().Pos}

	return &Binop{Left: exp, Op: "[]", Right: at, textRange: textRange}, nil
}

func (p *Parser) parseDotRest(exp Node) (Node, error) {
	p.scanner.Next()
	// parse index arguments
	attr, err := p.parseName(reservedKeywords...)
	if err != nil {
		return nil, err
	}
	textRange := TextRange{Start: exp.TextRange().Start, End: p.CurrentToken().Pos}
	return &DotOp{Left: exp, Attr: attr, textRange: textRange}, nil
}

func (p *Parser) simpleValue() (Node, error) {
	curr := p.CurrentToken()
	switch curr.Kind {
	case TokenName:
		return p.parseVar()
	case TokenNumber:
		return p.parseNumberNode()
	case TokenString:
		return p.parseStringNode()
	case TokenTemporal:
		return p.parseTemporalNode()
	default:
		return nil, p.Unexpected("name", "number", "string", "temporal")
	}
}

func (p *Parser) singleElement() (Node, error) {
	curr := p.CurrentToken()
	switch curr.Kind {
	case TokenName:
		return p.parseVar()
	// case TokenFuncall:
	// 	return p.parseFuncall()
	case TokenNumber:
		return p.parseNumberNode()
	case TokenString:
		return p.parseStringNode()
	case TokenTemporal:
		return p.parseTemporalNode()
	case "(":
		return p.parseBracketOrRange()
	case "[":
		return p.parseRangeOrArray()
	case "]":
		return p.parseOpenStartRange()
	case "{":
		return p.parseMapNode()
	case "?":
		return &Var{Name: "?"}, nil
	case TokenKeyword:
		switch curr.Value {
		case "true":
			return p.parseBool()
		case "false":
			return p.parseBool()
		case "null":
			return p.parseNull()
		case "if":
			return p.parseIfExpression()
		case "for":
			return p.parseForExpr(false)
		case "function":
			return p.parseFunDef()
		case "some":
			return p.parseSomeOrEvery()
		case "every":
			return p.parseSomeOrEvery()
		default:
			//return nil, p.Unexpected("keywords")
			// unexpected keywords can be part of names
			return p.parseVar()
		}
	default:
		return nil, p.Unexpected("name", "number", "string", "temporal", "(", "[", "keyword")
	}
}

// specialNameKeywordPrefixes lists the first word of the standard FEEL
// built-in names that legitimately embed a reserved word ("date and time",
// "years and months duration"). Reserved words are only folded into a
// multi-word variable name when the name parsed so far is one of these
// prefixes; otherwise a keyword like "and"/"or" ends the name so it can be
// parsed as the binary operator instead (e.g. `A and B`).
var specialNameKeywordPrefixes = map[string]bool{
	"date":  true, // date and time(...)
	"years": true, // years and months duration(...)
}

// reservedKeywords are the keywords recognized by the scanner (see the
// TokenKeyword pattern in scan.go). Passed as stopKeywords to parseName
// when parsing a dotted attribute name, so a reserved word immediately
// following the attribute (e.g. "settings.goldDiscount else ...") ends the
// name instead of being folded into it.
var reservedKeywords = []string{
	"true", "false", "and", "or", "null", "function", "if", "then", "else",
	"loop", "for", "some", "every", "in", "return", "satisfies",
}

func (p *Parser) parseVar() (Node, error) {
	textRange := p.startTextRange()
	names := make([]string, 0, 1)
	for p.CurrentToken().Expect(TokenName, TokenKeyword) {
		tok := p.CurrentToken()
		if tok.Kind == TokenKeyword && len(names) > 0 && !specialNameKeywordPrefixes[names[0]] {
			break
		}
		names = append(names, tok.Value)
		p.scanner.Next()
	}
	if len(names) == 0 {
		return nil, p.Unexpected(TokenName)
	}
	textRange.End = p.CurrentToken().Pos
	return &Var{Name: strings.Join(names, " "), textRange: textRange}, nil
}

func (p *Parser) parseBool() (Node, error) {
	textRange := p.startTextRange()
	v := p.CurrentToken().Value
	p.scanner.Next()
	textRange.End = p.CurrentToken().Pos
	switch v {
	case "true":
		return &BoolNode{Value: true, textRange: textRange}, nil
	case "false":
		return &BoolNode{Value: false, textRange: textRange}, nil
	default:
		return nil, p.Unexpected("true", "false")
	}
}

func (p *Parser) parseNull() (Node, error) {
	textRange := p.startTextRange()
	p.scanner.Next()
	textRange.End = p.CurrentToken().Pos
	return &NullNode{textRange: textRange}, nil
}

func containsKeywords(keywords []string, kw string) bool {
	return slices.Contains(keywords, kw)
}

func (p *Parser) parseName(stopKeywords ...string) (string, error) {
	names := make([]string, 0)

	for p.CurrentToken().Expect(TokenName, TokenKeyword) {
		if p.CurrentToken().Kind == "name" {
			names = append(names, p.CurrentToken().Value)
			p.scanner.Next()
		} else if p.CurrentToken().Kind == TokenKeyword {
			// keyworlds
			//if p.CurrentToken()
			kwVal := p.CurrentToken().Value
			if len(names) > 0 && containsKeywords(stopKeywords, kwVal) {
				break
			} else {
				names = append(names, kwVal)
				p.scanner.Next()
			}
		} else {
			break
		}
	}
	if len(names) <= 0 {
		return "", p.Unexpected(TokenName)
	}
	return strings.Join(names, " "), nil
}

func (p *Parser) parseBracketOrRange() (Node, error) {
	textRange := p.startTextRange()
	p.scanner.Next()
	if p.CurrentToken().Expect(">", ">=", "<", "<=", "!=", "=") {
		op := p.CurrentToken().Kind
		p.scanner.Next()
		val, err := p.expression()
		if err != nil {
			return nil, err
		}
		if !p.CurrentToken().Expect(")") {
			return nil, p.Unexpected(")")
		}
		p.scanner.Next()
		textRange.End = p.CurrentToken().Pos
		return &UnaryTestValueNode{Op: op, Value: val, textRange: textRange}, nil
	}
	c, err := p.expression()
	if err != nil {
		return nil, err
	}
	if p.CurrentToken().Kind == ".." {
		p.scanner.Next()
		p.inRangeEnd = true
		d, err := p.expression()
		p.inRangeEnd = false
		if err != nil {
			return nil, err
		}

		if p.CurrentToken().Kind == ")" || p.CurrentToken().Kind == "[" {
			p.scanner.Next()
			textRange.End = p.CurrentToken().Pos
			return &RangeNode{StartOpen: true, Start: c, EndOpen: true, End: d, textRange: textRange}, nil
		} else if p.CurrentToken().Kind == "]" {
			p.scanner.Next()
			textRange.End = p.CurrentToken().Pos
			return &RangeNode{StartOpen: true, Start: c, EndOpen: false, End: d, textRange: textRange}, nil
		}
		return nil, p.Unexpected(")", "]", "[")
	} else if p.CurrentToken().Expect(")") {
		p.scanner.Next()
	} else {
		return nil, p.Unexpected(")")
	}
	return c, nil
}

func (p *Parser) parseOpenStartRange() (Node, error) {
	rng := p.startTextRange()
	p.scanner.Next() // consume ']'
	start, err := p.expression()
	if err != nil {
		return nil, err
	}
	if !p.CurrentToken().Expect("..") {
		return nil, p.Unexpected("..")
	}
	p.scanner.Next()
	p.inRangeEnd = true
	end, err := p.expression()
	p.inRangeEnd = false
	if err != nil {
		return nil, err
	}
	endOpen := p.CurrentToken().Expect(")", "[")
	if !p.CurrentToken().Expect("]", ")", "[") {
		return nil, p.Unexpected("]", ")", "[")
	}
	p.scanner.Next()
	rng.End = p.CurrentToken().Pos
	return &RangeNode{StartOpen: true, Start: start, EndOpen: endOpen, End: end, textRange: rng}, nil
}

func (p *Parser) parseRangeOrArray() (Node, error) {
	rng := p.startTextRange()
	prefixKind := p.CurrentToken().Kind // prefixKind is '['
	p.scanner.Next()
	if p.CurrentToken().Expect("]") {
		p.scanner.Next()
		// empty array
		return &ArrayNode{}, nil
	}
	c, err := p.expression()
	if err != nil {
		return nil, err
	}

	if p.CurrentToken().Expect(",", "]") {
		return p.parseArrayGivenFirst(prefixKind, c)
	}

	if !p.CurrentToken().Expect("..") {
		return nil, p.Unexpected("..")
	}
	p.scanner.Next()
	p.inRangeEnd = true
	d, err := p.expression()
	p.inRangeEnd = false
	if err != nil {
		return nil, err
	}

	startOpen := prefixKind == "("
	if p.CurrentToken().Kind == ")" || p.CurrentToken().Kind == "[" {
		p.scanner.Next()
		rng.End = p.CurrentToken().Pos
		return &RangeNode{StartOpen: startOpen, Start: c, EndOpen: true, End: d, textRange: rng}, nil
	} else if p.CurrentToken().Kind == "]" {
		p.scanner.Next()
		rng.End = p.CurrentToken().Pos
		return &RangeNode{StartOpen: startOpen, Start: c, EndOpen: false, End: d, textRange: rng}, nil
	}
	return nil, p.Unexpected(")", "]", "[")
}

func (p *Parser) parseArrayGivenFirst(prefixKind string, firstElem Node) (Node, error) {
	rng := p.startTextRange()
	elements := []Node{firstElem}
	for p.CurrentToken().Expect(",") {
		p.scanner.Next()
		elem, err := p.expression()
		if err != nil {
			return nil, err
		}
		elements = append(elements, elem)
	}
	if !p.CurrentToken().Expect("]") {
		return nil, p.Unexpected("]")
	}
	p.scanner.Next()
	rng.End = p.CurrentToken().Pos
	return &ArrayNode{Elements: elements, textRange: rng}, nil
}

func (p *Parser) parseNumberNode() (Node, error) {
	rng := p.startTextRange()
	v := p.CurrentToken().Value
	p.scanner.Next()
	rng.End = p.CurrentToken().Pos
	return &NumberNode{Value: v, textRange: rng}, nil
}

func (p *Parser) parseStringNode() (Node, error) {
	rng := p.startTextRange()
	v := p.CurrentToken().Value
	p.scanner.Next()
	rng.End = p.CurrentToken().Pos
	return &StringNode{Value: v, textRange: rng}, nil
}

// mapKeyExtraSymbols lists the operator/punctuation tokens that FEEL
// permits inside a context entry key name (e.g. `{foo+bar: "foo"}`) as
// long as they are not separated from the surrounding name parts by
// whitespace.
var mapKeyExtraSymbols = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "**": true, "%": true, ".": true, "'": true,
}

func (p *Parser) parseMapKey() (string, error) {
	switch p.CurrentToken().Kind {
	case TokenName:
		return p.parseMapKeyName()
	case TokenString:
		node, err := p.parseStringNode()
		if err != nil {
			return "", err
		}
		return node.(*StringNode).Content(), nil
	default:
		return "", p.Unexpected(TokenName, TokenString)
	}
}

// parseMapKeyName parses a context entry key, which is normally a
// whitespace-separated multi-word name (`foo bar`) but may also embed
// operator symbols directly adjacent to name parts (`foo+bar`), per the
// FEEL grammar's "extra chars permitted in name" allowance.
func (p *Parser) parseMapKeyName() (string, error) {
	var sb strings.Builder
	var prevPos ScanPosition
	var prevLen int
	first := true
	for {
		tok := p.CurrentToken()
		isNamePart := tok.Kind == TokenName || tok.Kind == TokenKeyword || mapKeyExtraSymbols[tok.Kind]
		if !isNamePart {
			break
		}
		if !first && tok.Pos.Row == prevPos.Row && tok.Pos.Column != prevPos.Column+prevLen {
			sb.WriteString(" ")
		}
		sb.WriteString(tok.Value)
		prevPos = tok.Pos
		prevLen = len(tok.Value)
		first = false
		p.scanner.Next()
	}
	if sb.Len() == 0 {
		return "", p.Unexpected(TokenName)
	}
	return sb.String(), nil
}

func (p *Parser) parseTemporalNode() (Node, error) {
	rng := p.startTextRange()
	v := p.CurrentToken().Value
	p.scanner.Next()
	rng.End = p.CurrentToken().Pos
	return &TemporalNode{Value: v, textRange: rng}, nil
}

func (p *Parser) parseMapNode() (Node, error) {
	rng := p.startTextRange()
	p.scanner.Next()
	var mapValues []mapItem

	for !p.CurrentToken().Expect("}") {
		key, err := p.parseMapKey()
		if err != nil {
			return nil, err
		}

		if !p.CurrentToken().Expect(":") {
			return nil, p.Unexpected(":")
		}
		p.scanner.Next()

		exp, err := p.expression()
		if err != nil {
			return nil, err
		}

		mapValues = append(mapValues, mapItem{Name: key, Value: exp})

		if p.CurrentToken().Expect(",") {
			p.scanner.Next()
		} else if !p.CurrentToken().Expect("}") {
			return nil, p.Unexpected(",", "}")
		}
	}
	if p.CurrentToken().Expect("}") {
		p.scanner.Next()
	}
	rng.End = p.CurrentToken().Pos
	return &MapNode{Values: mapValues, textRange: rng}, nil
}

func (p *Parser) parseIfExpression() (Node, error) {
	rng := p.startTextRange()
	p.scanner.Next()
	cond, err := p.expression()
	if err != nil {
		return nil, err
	}
	if !p.CurrentToken().ExpectKeywords("then") {
		return nil, p.Unexpected("then")
	}
	p.scanner.Next()

	then_branch, err := p.expression()
	if err != nil {
		return nil, err
	}
	if !p.CurrentToken().ExpectKeywords("else") {
		return nil, p.Unexpected("else")
	}
	p.scanner.Next()

	else_branch, err := p.expression()
	if err != nil {
		return nil, err
	}

	rng.End = p.CurrentToken().Pos
	return &IfExpr{Cond: cond, ThenBranch: then_branch, ElseBranch: else_branch, textRange: rng}, nil

}

// parseIterationSource parses the value a "for"/"some"/"every" clause
// iterates over: either a plain list-valued expression, or a bare
// "start..end" range endpoint pair (no enclosing brackets), e.g.
// "for i in 2..4 return i".
func (p *Parser) parseIterationSource() (Node, error) {
	rng := p.startTextRange()
	first, err := p.expression()
	if err != nil {
		return nil, err
	}
	if p.CurrentToken().Kind != ".." {
		return first, nil
	}
	p.scanner.Next()
	p.inRangeEnd = true
	end, err := p.expression()
	p.inRangeEnd = false
	if err != nil {
		return nil, err
	}
	rng.End = p.CurrentToken().Pos
	return &RangeNode{Start: first, End: end, IterationRange: true, textRange: rng}, nil
}

func (p *Parser) parseForExpr(chained bool) (Node, error) {
	rng := p.startTextRange()
	p.scanner.Next()
	varName, err := p.parseName("in", "for")
	if err != nil {
		return nil, err
	}

	if !p.CurrentToken().ExpectKeywords("in") {
		return nil, p.Unexpected("in")
	}
	p.scanner.Next()

	listExpr, err := p.parseIterationSource()
	if err != nil {
		return nil, err
	}
	//fmt.Printf("list expr %s\n", listExpr.Repr())

	if p.CurrentToken().Expect(",") {
		returnExpr, err := p.parseForExpr(true)
		if err != nil {
			return nil, err
		}
		return &ForExpr{
			Varname:    varName,
			ListExpr:   listExpr,
			ReturnExpr: returnExpr,
			Chained:    chained,
		}, nil
	}

	if !p.CurrentToken().ExpectKeywords("return") {
		return nil, p.Unexpected("return")
	}
	p.scanner.Next()
	//fmt.Printf("return\n")

	returnExpr, err := p.expression()
	if err != nil {
		return nil, err
	}
	rng.End = p.CurrentToken().Pos
	return &ForExpr{
		Varname:    varName,
		ListExpr:   listExpr,
		ReturnExpr: returnExpr,
		Chained:    chained,
		textRange:  rng,
	}, nil
}

func (p *Parser) parseSomeOrEvery() (Node, error) {
	rng := p.startTextRange()
	cmd := p.CurrentToken().Value
	p.scanner.Next()
	// parse variable name
	varName, err := p.parseName("in")
	if err != nil {
		return nil, err
	}

	if !p.CurrentToken().ExpectKeywords("in") {
		return nil, p.Unexpected("in")
	}
	p.scanner.Next()

	listExpr, err := p.parseIterationSource()
	if err != nil {
		return nil, err
	}

	if !p.CurrentToken().ExpectKeywords("satisfies") {
		return nil, p.Unexpected("satisfies")
	}
	p.scanner.Next()

	filterExpr, err := p.expression()
	if err != nil {
		return nil, err
	}
	rng.End = p.CurrentToken().Pos
	if cmd == "some" {
		return &SomeExpr{
			Varname:    varName,
			ListExpr:   listExpr,
			FilterExpr: filterExpr,
			textRange:  rng,
		}, nil
	} else {
		return &EveryExpr{
			Varname:    varName,
			ListExpr:   listExpr,
			FilterExpr: filterExpr,
			textRange:  rng,
		}, nil
	}
}

func (p *Parser) parseFunDef() (Node, error) {
	rng := p.startTextRange()
	p.scanner.Next()
	if !p.CurrentToken().Expect("(") {
		return nil, p.Unexpected("(")
	}
	p.scanner.Next()

	// parse var list
	var args []string
	for !p.CurrentToken().Expect(")") {
		argName, err := p.parseName()
		if err != nil {
			return nil, err
		}

		args = append(args, argName)

		// Optional ": typeRef" annotation (e.g. "function(a: number, b: list<string>) ...").
		// The type isn't needed for evaluation, so just skip past it.
		if p.CurrentToken().Expect(":") {
			p.scanner.Next()
			depth := 0
			for {
				if p.CurrentToken().Expect("<") {
					depth++
				} else if p.CurrentToken().Expect(">") {
					depth--
				} else if depth == 0 && (p.CurrentToken().Expect(",") || p.CurrentToken().Expect(")")) {
					break
				} else if p.CurrentToken().Kind == TokenEOF {
					return nil, p.Unexpected(")", ",")
				}
				p.scanner.Next()
			}
		}

		if p.CurrentToken().Expect(",") {
			p.scanner.Next()
		} else if !p.CurrentToken().Expect(")") {
			return nil, p.Unexpected(")", ",")
		}
	}
	if isdup, name := hasDupName(args); isdup {
		return nil, fmt.Errorf("function arg name '%s' duplicates", name)
	}

	if p.CurrentToken().Expect(")") {
		p.scanner.Next()
	}

	exp, err := p.expression()
	if err != nil {
		return nil, err
	}
	rng.End = p.CurrentToken().Pos
	return &FunDef{
		Args:      args,
		Body:      exp,
		textRange: rng,
	}, nil
}
