package api

import "testing"

func TestRCFRationalResultFeedsAlgebraicNumeral(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	a, err := f.c.RCFRational("3/2")
	astTestMust(t, err)
	defer func() { astTestMust(t, a.Close()) }()
	b, err := f.c.RCFRational("2/3")
	astTestMust(t, err)
	defer func() { astTestMust(t, b.Close()) }()
	sum, err := f.c.RCFAdd(a, b)
	astTestMust(t, err)
	defer func() { astTestMust(t, sum.Close()) }()
	text, err := f.c.RCFString(sum)
	astTestMust(t, err)
	sort := f.pin(f.c.RealSort())
	value := f.pin(f.c.Numeral(text, sort))
	result := f.pin(f.c.AlgebraicMul(value, f.two))
	got, err := f.c.NumeralString(result)
	astTestMust(t, err)
	if got != "13/3" {
		t.Fatalf("algebraic product: %s", got)
	}
}
