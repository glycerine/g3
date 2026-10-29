package realclosure

import "testing"

func TestRefineSingleAlgebraicRootToPrecision(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	root := roots[1].root
	refined, err := f.store.refineSingleAlgebraicRoot(root, 12)
	if err != nil || !refined {
		t.Fatal(refined, err)
	}
	precise, err := binaryIntervalPrecise(root.interval, 12)
	if err != nil || !precise {
		t.Fatal(precise, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
