package realclosure

import "testing"

func expectRationalPolynomial(t *testing.T, f *boundedSignFixture, got []*realValue, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("polynomial length=%d want=%d", len(got), len(want))
	}
	for i, n := range want {
		if n == 0 {
			if got[i] != nil {
				t.Fatalf("coefficient %d is nonzero", i)
			}
			continue
		}
		expected := f.rational(n)
		if got[i] == nil || !got[i].isRational {
			t.Fatalf("coefficient %d is not rational", i)
		}
		equal, err := f.store.numbers.Equal(&got[i].rational, &expected.rational)
		if err != nil || !equal {
			t.Fatalf("coefficient %d mismatch err=%v", i, err)
		}
	}
}

func TestRationalPolynomialDivisionGCDAndSquareFree(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newBoundedSignFixture(t, -3, 3)
			two, minusThree, one, minusOne := f.rational(2), f.rational(-3), f.rational(1), f.rational(-1)
			p := []*realValue{two, minusThree, nil, one} // (x-1)^2(x+2)
			factor := []*realValue{minusOne, one}
			quotient, remainder, err := f.store.divideRationalPolynomial(p, factor)
			if err != nil || len(remainder) != 0 {
				t.Fatal("exact division", len(remainder), err)
			}
			expectRationalPolynomial(t, f, quotient, -2, 1, 1)
			if err := f.store.releaseSturmCoefficients(quotient); err != nil {
				t.Fatal(err)
			}
			derivative, err := f.store.derivativeRationalPolynomial(p)
			if err != nil {
				t.Fatal(err)
			}
			gcd, err := f.store.gcdRationalPolynomials(p, derivative, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			expectRationalPolynomial(t, f, gcd, -1, 1)
			if err := f.store.releaseSturmCoefficients(derivative); err != nil {
				t.Fatal(err)
			}
			if err := f.store.releaseSturmCoefficients(gcd); err != nil {
				t.Fatal(err)
			}
			squareFree, err := f.store.squareFreeRationalPolynomial(p, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			expectRationalPolynomial(t, f, squareFree, -2, 1, 1)
			if err := f.store.releaseSturmCoefficients(squareFree); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRationalPolynomialDivisionKeepsRemainder(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	one, two := f.rational(1), f.rational(2)
	quotient, remainder, err := f.store.divideRationalPolynomial([]*realValue{one, nil, one}, []*realValue{two, one})
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, quotient, -2, 1)
	expectRationalPolynomial(t, f, remainder, 5)
	if err := f.store.releaseSturmCoefficients(quotient); err != nil {
		t.Fatal(err)
	}
	if err := f.store.releaseSturmCoefficients(remainder); err != nil {
		t.Fatal(err)
	}
}
