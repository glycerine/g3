package api

import "testing"

func TestAPIAlgebraicRationalAndMixedArithmetic(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	sum, err := f.c.AlgebraicAdd(root, f.one)
	astTestMust(t, err)
	sum = f.pin(sum, nil)
	restored, err := f.c.AlgebraicSub(sum, f.one)
	astTestMust(t, err)
	restored = f.pin(restored, nil)
	if equal, compareErr := f.c.AlgebraicEqual(root, restored); compareErr != nil || !equal {
		t.Fatal("mixed add/sub", equal, compareErr)
	}
	product, err := f.c.AlgebraicMul(root, root)
	astTestMust(t, err)
	product = f.pin(product, nil)
	text, err := f.c.NumeralString(product)
	astTestMust(t, err)
	if text != "2" {
		t.Fatal("sqrt2 product", text)
	}
	quotient, err := f.c.AlgebraicDiv(product, root)
	astTestMust(t, err)
	quotient = f.pin(quotient, nil)
	if equal, compareErr := f.c.AlgebraicEqual(root, quotient); compareErr != nil || !equal {
		t.Fatal("mixed division", equal, compareErr)
	}
	rational, err := f.c.AlgebraicAdd(f.half, f.half)
	astTestMust(t, err)
	rational = f.pin(rational, nil)
	if text, err = f.c.NumeralString(rational); err != nil || text != "1" {
		t.Fatal("rational result", text, err)
	}
}
