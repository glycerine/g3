package realclosure

import "testing"

func ambiguousSqrtDifference(t *testing.T, usePrem bool) (*boundedSignFixture, []*realValue, *realValue) {
	t.Helper()
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	threeHalves := fixtureFraction(t, f, 3, 2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	difference, err := f.store.subtractRetainedValues(roots[1], threeHalves)
	if err != nil {
		t.Fatal(err)
	}
	contains, err := difference.functionInterval.ContainsZero()
	if err != nil || !contains {
		t.Fatal("fixture is not ambiguous", contains, err)
	}
	return f, roots, difference
}

func TestExactSingleRootPolynomialSign(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		f, roots, difference := ambiguousSqrtDifference(t, usePrem)
		sign, err := f.store.exactSingleRootPolynomialSign(difference, usePrem)
		if err != nil || sign != -1 {
			t.Fatal(sign, err)
		}
		if err := f.store.release(difference); err != nil {
			t.Fatal(err)
		}
		releaseIsolatedValues(t, f.store, roots)
	}
}
