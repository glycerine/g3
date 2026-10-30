package realclosure

import "testing"

func TestRealValueAlgebraicRemainderReducesCubic(t *testing.T) {
	f := newRealFunctionFixture(t)
	reduced, err := f.store.reduceAlgebraicNumerator(f.root, []*realValue{nil, nil, nil, f.one})
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(reduced)) })
	if len(reduced) != 2 || reduced[0] != nil || f.coefficientText(reduced[1]) != "2" {
		t.Fatal("x^3 mod (x^2-2) did not equal 2*x")
	}
}
