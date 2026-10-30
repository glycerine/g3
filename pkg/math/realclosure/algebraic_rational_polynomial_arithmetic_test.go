package realclosure

import (
	"math/big"
	"testing"
)

func TestAlgebraicRationalPolynomialDivisionAndInverseIdentity(t *testing.T) {
	q := make([]big.Rat, 2)
	q[1].SetInt64(1)
	p := make([]big.Rat, 3)
	p[0].SetInt64(-2)
	p[2].SetInt64(1)
	h, ok := invertAlgebraicPolynomial(q, p)
	if !ok || len(h) != 2 || h[0].Sign() != 0 || h[1].RatString() != "1/2" {
		t.Fatal("extended gcd inverse", ok, h)
	}
	product := bigPolynomialMul(q, h)
	remainder, ok := bigPolynomialMod(product, p)
	if !ok || len(remainder) != 1 || remainder[0].RatString() != "1" {
		t.Fatal("inverse identity modulo definition", ok, remainder)
	}
}
