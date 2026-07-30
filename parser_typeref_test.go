package feel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Regression tests for the parser's optional ": typeRef" argument
// annotations (e.g. "function(a: number, b: list<string>) ..."). The type
// itself is discarded during parsing; these tests check that annotated
// function literals parse to the same AST as their unannotated form, and
// that the "<...>" depth tracking correctly ignores commas/">" inside a
// generic type when deciding where the type ends.

func TestFunDefParsesTypeAnnotations(t *testing.T) {
	assert := assert.New(t)

	ast, err := ParseString(`function(a: number, b: string) a + b`)
	assert.Nil(err)
	assert.Equal(`(function [a, b] (+ a b))`, ast.Repr())
}

func TestFunDefParsesGenericTypeAnnotations(t *testing.T) {
	assert := assert.New(t)

	// The "," inside list<string> must not be treated as an argument
	// separator, and depth must return to 0 by the time we hit ")".
	ast, err := ParseString(`function(a: list<string>, b: context<string, number>) a`)
	assert.Nil(err)
	assert.Equal(`(function [a, b] a)`, ast.Repr())
}

func TestFunDefParsesSingleTypeAnnotation(t *testing.T) {
	assert := assert.New(t)

	ast, err := ParseString(`function(a: number) a + 1`)
	assert.Nil(err)
	assert.Equal(`(function [a] (+ a 1))`, ast.Repr())
}

func TestFunDefUnterminatedTypeAnnotationErrors(t *testing.T) {
	assert := assert.New(t)

	_, err := ParseString(`function(a: number`)
	assert.NotNil(err)
	une, ok := err.(*UnexpectedToken)
	assert.True(ok)
	assert.Equal(TokenEOF, une.token.Kind)
	assert.Equal([]string{")", ","}, une.expects)
}

func TestFunDefWithTypeAnnotationsIsCallable(t *testing.T) {
	got := evalWithScope(t, Scope{}, `(function(a: number, b: number) a + b)(3, 4)`)
	assert.Equal(t, float64(7), numFloat(t, got))
}
