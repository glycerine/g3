package realclosure

import "testing"

func TestDeterminedComparisonUsesExactSignFallback(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	threeHalves := fixtureFraction(t, f, 3, 2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := f.store.compareDeterminedValues(roots[1], threeHalves, false)
	if err != nil || comparison != -1 {
		t.Fatal(comparison, err)
	}
	comparison, err = f.store.compareDeterminedValues(threeHalves, roots[1], false)
	if err != nil || comparison != 1 {
		t.Fatal(comparison, err)
	}
	comparison, err = f.store.compareDeterminedValues(roots[1], roots[1], false)
	if err != nil || comparison != 0 {
		t.Fatal(comparison, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
