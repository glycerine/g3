package realclosure

import "testing"

func TestNormalizeMixedAlgebraicIntegerContent(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one, six, nine := f.rational(-2), f.rational(1), f.rational(6), f.rational(9)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	sixAlpha, err := f.store.multiplyRetainedValues(roots[1], six)
	if err != nil {
		t.Fatal(err)
	}
	normalized, changed, err := f.store.normalizeAnyIntegerContent([]*realValue{sixAlpha, nine}, true)
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	expectRationalPolynomial(t, f, normalized[0].functionNumerator.coefficients, 0, 2)
	expectRationalPolynomial(t, f, normalized[1:], 3)
	if err := f.store.releaseSturmCoefficients(normalized); err != nil {
		t.Fatal(err)
	}
	if err := f.store.release(sixAlpha); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
