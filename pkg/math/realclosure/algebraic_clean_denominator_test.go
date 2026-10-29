package realclosure

import "testing"

func TestCleanAlgebraicValueDenominator(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one, two := f.rational(-2), f.rational(1), f.rational(2)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	halfAlpha, err := f.store.divideRetainedValues(roots[1], two)
	if err != nil {
		t.Fatal(err)
	}
	clean, err := f.store.hasCleanAnyDenominator(halfAlpha)
	if err != nil || clean {
		t.Fatal(clean, err)
	}
	numerator, denominator, err := f.store.cleanAnyValueDenominator(halfAlpha, true)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := f.store.structurallyEqualValues(numerator, roots[1])
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	expectRationalPolynomial(t, f, []*realValue{denominator}, 2)
	reconstructed, err := f.store.divideRetainedValues(numerator, denominator)
	if err != nil {
		t.Fatal(err)
	}
	equal, err = f.store.equalAnyDeterminedValues(reconstructed, halfAlpha, true)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	for _, value := range []*realValue{halfAlpha, numerator, denominator, reconstructed} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}
