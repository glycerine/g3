package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestNumeralRationalSourceFamiliesAndOwnership(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	real := f.pin(f.c.RealSort())
	bv := f.pin(f.c.BitVectorSort(8))
	finite := f.pin(f.c.FiniteDomainSort("rational-source-domain", 4))
	for _, tc := range []struct {
		term ast.ASTHandle
		want string
	}{
		{f.pin(f.c.Numeral("7/3", real)), "7/3"},
		{f.pin(f.c.UintNumeral(257, bv)), "1"},
		{f.pin(f.c.UintNumeral(3, finite)), "3"},
	} {
		value, found, err := f.c.NumeralRational(tc.term)
		astTestMust(t, err)
		if !found || value.RatString() != tc.want {
			t.Fatalf("numeral: %v, %v, want %s", value, found, tc.want)
		}
		value.SetInt64(100)
		again, found, err := f.c.NumeralRational(tc.term)
		astTestMust(t, err)
		if !found || again.RatString() != tc.want {
			t.Fatal("returned rational aliases stored numeral")
		}
	}
	variable := f.arithmeticVariable("rational-source-variable", false)
	if value, found, err := f.c.NumeralRational(variable); err != nil || found || value != nil {
		t.Fatal("nonnumeral", value, found, err)
	}
}
