package realclosure

import "testing"

func TestRetainedPrecisionAndMagnitudeQueries(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.refineAnyRetainedInterval(roots[1], f.bounds, 11); err != nil {
		t.Fatal(err)
	}
	precise, err := f.store.retainedIntervalPrecise(roots[1], f.bounds, 11)
	if err != nil || !precise {
		t.Fatal(precise, err)
	}
	magnitude, err := f.store.retainedIntervalMagnitude(roots[1], f.bounds)
	if err != nil || magnitude > -11 {
		t.Fatal(magnitude, err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
