package api

import "testing"

func TestAPINumeralDecimalStringExactNumerals(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	integer, err := f.c.NumeralDecimalString(f.three, 20)
	astTestMust(t, err)
	bv := f.pin(f.c.BitVectorNumeral([]bool{true, false, true}))
	bitVector, err := f.c.NumeralDecimalString(bv, 20)
	astTestMust(t, err)
	if integer != "3" || bitVector != "5" {
		t.Fatal("exact numeral decimals", integer, bitVector)
	}
}
