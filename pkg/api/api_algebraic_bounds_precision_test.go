package api

import (
	"math/big"
	"testing"
)

func TestAPIAlgebraicNumberBoundsRefineWithPrecision(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	lo0 := f.pin(f.c.AlgebraicNumberLower(root, 0))
	hi0 := f.pin(f.c.AlgebraicNumberUpper(root, 0))
	lo12 := f.pin(f.c.AlgebraicNumberLower(root, 12))
	hi12 := f.pin(f.c.AlgebraicNumberUpper(root, 12))
	lo0q, ok, err := f.c.NumeralRational(lo0)
	astTestMust(t, err)
	if !ok {
		t.Fatal("coarse lower bound is not rational")
	}
	hi0q, ok, err := f.c.NumeralRational(hi0)
	astTestMust(t, err)
	if !ok {
		t.Fatal("coarse upper bound is not rational")
	}
	lo12q, ok, err := f.c.NumeralRational(lo12)
	astTestMust(t, err)
	if !ok {
		t.Fatal("refined lower bound is not rational")
	}
	hi12q, ok, err := f.c.NumeralRational(hi12)
	astTestMust(t, err)
	if !ok {
		t.Fatal("refined upper bound is not rational")
	}
	coarse := new(big.Rat).Sub(hi0q, lo0q)
	refined := new(big.Rat).Sub(hi12q, lo12q)
	if refined.Sign() <= 0 || refined.Cmp(coarse) >= 0 {
		t.Fatal("precision did not narrow bounds", coarse, refined)
	}
}
