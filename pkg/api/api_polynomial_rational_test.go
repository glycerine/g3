package api

import "testing"

func TestAPIPolynomialSubresultantsRationalCoefficients(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	xSquared := f.pin(f.c.ArithmeticMul(f.x, f.x))
	halfSquare := f.pin(f.c.ArithmeticMul(f.half, xSquared))
	p := f.pin(f.c.ArithmeticSub(halfSquare, f.one))
	result, err := f.c.PolynomialSubresultants(p, f.q, f.x)
	astTestMust(t, err)
	defer func() { astTestMust(t, result.Close()) }()
	if size, sizeErr := result.Len(); sizeErr != nil || size == 0 {
		t.Fatal("rational coefficient chain", size, sizeErr)
	}
	for i, text := range polynomialResultStrings(t, f, result) {
		if text == "" {
			t.Fatal("empty reconstructed polynomial", i)
		}
	}
}
