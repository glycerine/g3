package realclosure

import "testing"

func TestDeterminedComparisonPredicates(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	threeHalves := fixtureFraction(t, f, 3, 2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	less, err := f.store.lessDeterminedValues(roots[1], threeHalves, true)
	if err != nil || !less {
		t.Fatal(less, err)
	}
	greater, err := f.store.greaterDeterminedValues(threeHalves, roots[1], true)
	if err != nil || !greater {
		t.Fatal(greater, err)
	}
	equal, err := f.store.equalDeterminedValues(roots[1], roots[1], true)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
