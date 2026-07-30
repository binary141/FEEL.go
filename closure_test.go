package feel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Regression tests for bad241c: a function value returned out of its
// defining function used to lose access to the scope it was created in,
// because FunDef.Eval didn't capture a closure and calls were evaluated
// against the caller's ambient scope instead.

func TestClosureCapturesDefiningScope(t *testing.T) {
	got := evalWithScope(t, Scope{}, `
		(function(x) function(y) x + y)(5)(3)
	`)
	assert.Equal(t, float64(8), numFloat(t, got))
}

func TestClosureSurvivesReturnFromDefiningFunction(t *testing.T) {
	// makeAdder returns a closure and then goes out of scope; calling the
	// returned function later must still see x = 5 from makeAdder's frame
	// rather than whatever happens to be on the caller's scope stack.
	got := evalWithScope(t, Scope{}, `
		{
			makeAdder: function(x) function(y) x + y,
			add5: makeAdder(5)
		}.add5(3)
	`)
	assert.Equal(t, float64(8), numFloat(t, got))
}

func TestClosureDoesNotLeakIntoCallerScope(t *testing.T) {
	// The closure's captured x must not be visible from the caller's own
	// scope after the call returns.
	got := evalWithScope(t, Scope{}, `
		{
			x: 100,
			makeAdder: function(x) function(y) x + y,
			add5: makeAdder(5),
			result: add5(3)
		}.result
	`)
	assert.Equal(t, float64(8), numFloat(t, got))
}

func TestClosureEachCallGetsIndependentCapture(t *testing.T) {
	// Two closures created from separate calls to makeAdder must each keep
	// their own captured x rather than sharing one mutable scope.
	got := evalWithScope(t, Scope{}, `
		{
			makeAdder: function(x) function(y) x + y,
			add5: makeAdder(5),
			add10: makeAdder(10)
		}.add5(1) + {
			makeAdder: function(x) function(y) x + y,
			add5: makeAdder(5),
			add10: makeAdder(10)
		}.add10(1)
	`)
	assert.Equal(t, float64(17), numFloat(t, got))
}
