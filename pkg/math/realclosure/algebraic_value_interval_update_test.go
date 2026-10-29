package realclosure

import "testing"

func TestUpdateAlgebraicPolynomialValueInterval(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.store.addRetainedValues(roots[1], one)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.refineSingleAlgebraicRoot(value.root, 10); err != nil {
		t.Fatal(err)
	}
	if err := f.store.updateAlgebraicPolynomialValueInterval(value); err != nil {
		t.Fatal(err)
	}
	precise, err := binaryIntervalPrecise(value.functionInterval, 9)
	if err != nil || !precise {
		t.Fatal(precise, err)
	}
	if err := f.store.release(value); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
