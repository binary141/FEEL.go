package feel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func evalWithScope(t *testing.T, scope Scope, input string) any {
	t.Helper()
	ast, err := ParseString(input)
	assert.Nil(t, err)
	intp := NewIntepreter()
	intp.Push(scope)
	got, err := ast.Eval(intp)
	assert.Nil(t, err)
	return got
}

func numFloat(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(*Number)
	assert.True(t, ok, "expected *Number, got %T", v)
	return n.Float64()
}

// Regression test for 5bf7c65: "context put" used to mutate the original
// context in place instead of returning a copy.
func TestContextPutDoesNotMutateOriginal(t *testing.T) {
	orig := map[string]any{"a": NewNumberFromInt64(1)}
	scope := Scope{"orig": orig}

	got := evalWithScope(t, scope, `context put(orig, "a", 2)`)

	newCtx, ok := got.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(2), numFloat(t, newCtx["a"]))
	assert.Equal(t, float64(1), numFloat(t, orig["a"]), "original context must not be mutated")
}

func TestContextPutNestedDoesNotMutateOriginal(t *testing.T) {
	inner := map[string]any{"a": NewNumberFromInt64(1)}
	orig := map[string]any{"x": inner}
	scope := Scope{"orig": orig}

	got := evalWithScope(t, scope, `context put(orig, ["x", "a"], 2)`)

	newCtx, ok := got.(map[string]any)
	assert.True(t, ok)
	newInner, ok := newCtx["x"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(2), numFloat(t, newInner["a"]))
	assert.Equal(t, float64(1), numFloat(t, inner["a"]), "original nested context must not be mutated")
}

func TestContextPutSingleKey(t *testing.T) {
	got := evalWithScope(t, Scope{}, `context put({a: 1}, "b", 2)`)
	newCtx, ok := got.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, newCtx["a"]))
	assert.Equal(t, float64(2), numFloat(t, newCtx["b"]))
}

func TestContextPutKeyList(t *testing.T) {
	got := evalWithScope(t, Scope{}, `context put({x: {a: 1}}, ["x", "b"], 2)`)
	newCtx, ok := got.(map[string]any)
	assert.True(t, ok)
	x, ok := newCtx["x"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, x["a"]))
	assert.Equal(t, float64(2), numFloat(t, x["b"]))
}
