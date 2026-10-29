package realclosure

import "testing"

func TestRetainedAlgebraicInverseAndDivision(t *testing.T) {
	f := newBoundedSignFixture(t, -3, 3)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	alpha := roots[1]
	inverse, err := f.store.inverseRetainedValue(alpha)
	if err != nil || inverse == nil || inverse.isRational {
		t.Fatal(inverse, err)
	}
	if inverse.functionNumerator.coefficients[0] != nil {
		t.Fatal("inverse constant coefficient")
	}
	half := fixtureFraction(t, f, 1, 2)
	equal, err := f.store.numbers.Equal(&inverse.functionNumerator.coefficients[1].rational, &half.rational)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	checkOne, err := f.store.multiplyRetainedValues(alpha, inverse)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{checkOne}, 1)
	alphaPlusOne, err := f.store.addRetainedValues(alpha, one)
	if err != nil {
		t.Fatal(err)
	}
	quotient, err := f.store.divideRetainedValues(alpha, alphaPlusOne)
	if err != nil || quotient == nil || quotient.isRational {
		t.Fatal(quotient, err)
	}
	expectRationalPolynomial(t, f, quotient.functionNumerator.coefficients, 2, -1)
	reconstructed, err := f.store.multiplyRetainedValues(quotient, alphaPlusOne)
	if err != nil {
		t.Fatal(err)
	}
	equalValue, err := f.store.structurallyEqualValues(reconstructed, alpha)
	if err != nil || !equalValue {
		t.Fatal(equalValue, err)
	}
	for _, value := range []*realValue{inverse, checkOne, alphaPlusOne, quotient, reconstructed} {
		if err := f.store.release(value); err != nil {
			t.Fatal(err)
		}
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestRetainedRationalInverseAndDivisionGuards(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	two, four := f.rational(2), f.rational(4)
	inverse, err := f.store.inverseRetainedValue(two)
	if err != nil {
		t.Fatal(err)
	}
	half := fixtureFraction(t, f, 1, 2)
	equal, err := f.store.numbers.Equal(&inverse.rational, &half.rational)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	quotient, err := f.store.divideRetainedValues(four, two)
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{quotient}, 2)
	if _, err := f.store.divideRetainedValues(two, nil); err == nil {
		t.Fatal("division by zero accepted")
	}
	if err := f.store.release(inverse); err != nil {
		t.Fatal(err)
	}
	if err := f.store.release(quotient); err != nil {
		t.Fatal(err)
	}
}
