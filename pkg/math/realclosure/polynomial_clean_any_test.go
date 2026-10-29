package realclosure

import "testing"

func TestCleanMixedAlgebraicPolynomialDenominators(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one, two := f.rational(-2), f.rational(1), f.rational(2)
	third := fixtureFraction(t, f, 1, 3)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	halfAlpha, err := f.store.divideRetainedValues(roots[1], two)
	if err != nil {
		t.Fatal(err)
	}
	normalized, denominator, err := f.store.cleanAnyPolynomialDenominators([]*realValue{halfAlpha, third}, false)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{denominator}, 6)
	threeAlpha, err := f.store.multiplyRetainedValues(roots[1], f.rational(3))
	if err != nil {
		t.Fatal(err)
	}
	equal, err := f.store.structurallyEqualValues(normalized[0], threeAlpha)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	expectRationalPolynomial(t, f, normalized[1:], 2)
	for _, value := range []*realValue{halfAlpha, denominator, threeAlpha} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.store.releaseSturmCoefficients(normalized); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
