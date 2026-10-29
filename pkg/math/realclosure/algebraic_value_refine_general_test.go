package realclosure

import "testing"

func TestRefineGeneralAlgebraicPolynomialValue(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.store.addRetainedValues(roots[1], one)
	if err != nil {
		t.Fatal(err)
	}
	refined, err := f.store.refineAlgebraicPolynomialValue(value, 13)
	if err != nil || !refined {
		t.Fatal(refined, err)
	}
	precise, err := binaryIntervalPrecise(value.functionInterval, 13)
	if err != nil || !precise {
		t.Fatal(precise, err)
	}
	if err := f.store.release(value); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
