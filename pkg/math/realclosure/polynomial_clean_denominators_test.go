package realclosure

import "testing"

func TestCleanRationalPolynomialDenominatorsLCM(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	p := []*realValue{
		fixtureFraction(t, f, 1, 2),
		fixtureFraction(t, f, -2, 3),
		fixtureFraction(t, f, 3, 4),
	}
	cleanBefore, err := f.store.hasCleanPolynomialDenominators(p)
	if err != nil || cleanBefore {
		t.Fatal(cleanBefore, err)
	}
	normalized, denominator, err := f.store.cleanRationalPolynomialDenominators(p)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, normalized, 6, -8, 9)
	expectRationalPolynomial(t, f, []*realValue{denominator}, 12)
	cleanAfter, err := f.store.hasCleanPolynomialDenominators(normalized)
	if err != nil || !cleanAfter {
		t.Fatal(cleanAfter, err)
	}
	if err := f.store.releaseSturmCoefficients(normalized); err != nil {
		t.Fatal(err)
	}
	if err := f.store.release(denominator); err != nil {
		t.Fatal(err)
	}
}

func TestCleanRationalPolynomialPreservesZeroSlots(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	p := []*realValue{nil, fixtureFraction(t, f, 5, 6)}
	normalized, denominator, err := f.store.cleanRationalPolynomialDenominators(p)
	if err != nil {
		t.Fatal(err)
	}
	if normalized[0] != nil {
		t.Fatal("zero slot became allocated")
	}
	expectRationalPolynomial(t, f, normalized, 0, 5)
	expectRationalPolynomial(t, f, []*realValue{denominator}, 6)
	if err := f.store.releaseSturmCoefficients(normalized); err != nil {
		t.Fatal(err)
	}
	if err := f.store.release(denominator); err != nil {
		t.Fatal(err)
	}
}
