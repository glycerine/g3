package api

import "testing"

func TestAPIPolynomialSubresultantsNonMappedVariableIsEmpty(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	result, err := f.c.PolynomialSubresultants(f.p, f.q, f.y)
	astTestMust(t, err)
	defer func() { astTestMust(t, result.Close()) }()
	if size, sizeErr := result.Len(); sizeErr != nil || size != 0 {
		t.Fatal("non-variable result", size, sizeErr)
	}
}
