package realclosure

import "testing"

func TestRetainedAlgebraicPowerBySquaring(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	alpha := roots[1]
	zeroPower, err := f.store.powerRetainedValue(alpha, 0)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{zeroPower}, 1)
	square, err := f.store.powerRetainedValue(alpha, 2)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{square}, 2)
	cube, err := f.store.powerRetainedValue(alpha, 3)
	if err != nil || cube == nil || cube.isRational {
		t.Fatal(cube, err)
	}
	expectRationalPolynomial(t, f, cube.functionNumerator.coefficients, 0, 2)
	zeroCube, err := f.store.powerRetainedValue(nil, 3)
	if err != nil || zeroCube != nil {
		t.Fatal(zeroCube, err)
	}
	for _, value := range []*realValue{zeroPower, square, cube} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestAlgebraicPolynomialReductionCubicNumerator(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	reduced, err := f.store.reduceAlgebraicPolynomial(roots[1].root, []*realValue{nil, nil, nil, one})
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, reduced, 0, 2)
	if err := f.store.releaseSturmCoefficients(reduced); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
