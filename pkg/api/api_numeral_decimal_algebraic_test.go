package api

import (
	"strings"
	"testing"
)

func TestAPINumeralDecimalStringAlgebraic(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	coarse, err := f.c.NumeralDecimalString(root, 3)
	astTestMust(t, err)
	fine, err := f.c.NumeralDecimalString(root, 12)
	astTestMust(t, err)
	if coarse != "1.414?" || !strings.HasPrefix(fine, "1.41421356237") || len(fine) <= len(coarse) {
		t.Fatal("algebraic decimals", coarse, fine)
	}
}
