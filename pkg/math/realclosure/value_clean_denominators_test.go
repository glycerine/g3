package realclosure

import "testing"

func fixtureFraction(t *testing.T, f *boundedSignFixture, numerator int64, denominator uint64) *realValue {
	t.Helper()
	value := f.rational(1)
	if err := f.store.numbers.SetFraction(&value.rational, numerator, denominator); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestCleanRationalValueDenominator(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	twoThirds := fixtureFraction(t, f, 2, 3)
	numerator, denominator, err := f.store.cleanRationalValueDenominator(twoThirds)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{numerator, denominator}, 2, 3)
	for _, value := range []*realValue{numerator, denominator} {
		clean, cleanErr := f.store.hasCleanDenominator(value)
		if cleanErr != nil || !clean {
			t.Fatal(clean, cleanErr)
		}
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	zeroNumerator, oneDenominator, err := f.store.cleanRationalValueDenominator(nil)
	if err != nil || zeroNumerator != nil {
		t.Fatal(zeroNumerator, err)
	}
	expectRationalPolynomial(t, f, []*realValue{oneDenominator}, 1)
	if err := f.store.release(oneDenominator); err != nil {
		t.Fatal(err)
	}
}
