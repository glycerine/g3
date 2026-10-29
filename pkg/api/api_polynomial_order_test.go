package api

import (
	"testing"
)

func TestAPIPolynomialSubresultantsSwapPreservesSourceChain(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	forward, err := f.c.PolynomialSubresultants(f.p, f.q, f.x)
	astTestMust(t, err)
	defer func() { astTestMust(t, forward.Close()) }()
	reverse, err := f.c.PolynomialSubresultants(f.q, f.p, f.x)
	astTestMust(t, err)
	defer func() { astTestMust(t, reverse.Close()) }()
	left := polynomialResultStrings(t, f, forward)
	right := polynomialResultStrings(t, f, reverse)
	equal := len(left) == len(right)
	for i := range left {
		equal = equal && left[i] == right[i]
	}
	if !equal {
		t.Fatal("swapped chain", left, right)
	}
}
