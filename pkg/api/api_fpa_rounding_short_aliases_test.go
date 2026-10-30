package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointRoundingShortAliasesCanonical(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	rows := []struct {
		short func() (ast.ASTHandle, error)
		long  func() (ast.ASTHandle, error)
	}{
		{f.context.FloatingPointRNE, f.context.FloatingPointRoundNearestEven},
		{f.context.FloatingPointRNA, f.context.FloatingPointRoundNearestAway},
		{f.context.FloatingPointRTP, f.context.FloatingPointRoundTowardPositiveValue},
		{f.context.FloatingPointRTN, f.context.FloatingPointRoundTowardNegativeValue},
		{f.context.FloatingPointRTZ, f.context.FloatingPointRoundTowardZeroValue},
	}
	for index, row := range rows {
		short, err := row.short()
		short = f.pin(t, short, err)
		long, err := row.long()
		long = f.pin(t, long, err)
		if short != long {
			t.Fatal("noncanonical rounding alias", index, short, long)
		}
	}
}
