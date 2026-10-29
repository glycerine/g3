package api

import "testing"

func TestAPIPolynomialSubresultantsBasicAndEqualInputs(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	result, err := f.c.PolynomialSubresultants(f.p, f.q, f.x)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, result.Close()) })
	size, err := result.Len()
	astTestMust(t, err)
	if size == 0 || len(polynomialResultStrings(t, f, result)) != int(size) {
		t.Fatal("subresultant chain size", size)
	}
	equal, err := f.c.PolynomialSubresultants(f.p, f.p, f.x)
	astTestMust(t, err)
	defer func() { astTestMust(t, equal.Close()) }()
	if n, sizeErr := equal.Len(); sizeErr != nil || n != 1 {
		t.Fatal("equal-input chain", n, sizeErr)
	}
	h, err := equal.Get(0)
	astTestMust(t, err)
	if text, textErr := f.c.NumeralString(h); textErr != nil || text != "0" {
		t.Fatal("equal-input zero", text, textErr)
	}
}
