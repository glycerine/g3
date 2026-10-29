package realclosure

import "testing"

func TestRationalPolynomialModularInverse(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	inverse, coprime, err := f.store.invertRationalPolynomialModulo([]*realValue{nil, one}, []*realValue{minusTwo, nil, one})
	if err != nil || !coprime || len(inverse) != 2 || inverse[0] != nil {
		t.Fatal(inverse, coprime, err)
	}
	half := fixtureFraction(t, f, 1, 2)
	equal, err := f.store.numbers.Equal(&inverse[1].rational, &half.rational)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	product, err := f.store.multiplyRationalPolynomials([]*realValue{nil, one}, inverse)
	if err != nil {
		t.Fatal(err)
	}
	quotient, reduced, err := f.store.divideRationalPolynomial(product, []*realValue{minusTwo, nil, one})
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, reduced, 1)
	for _, polynomial := range [][]*realValue{inverse, product, quotient, reduced} {
		if err := f.store.releaseSturmCoefficients(polynomial); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRationalPolynomialModularInverseDetectsFactor(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusOne, one := f.rational(-1), f.rational(1)
	inverse, coprime, err := f.store.invertRationalPolynomialModulo([]*realValue{minusOne, one}, []*realValue{minusOne, nil, one})
	if err != nil || coprime || inverse != nil {
		t.Fatal(inverse, coprime, err)
	}
}
