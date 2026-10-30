package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestFloatingPointRoundingShortAliasModes(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	rows := []struct {
		make func() (ast.ASTHandle, error)
		want unumeral.ArbitraryFloatRoundingMode
	}{
		{f.context.FloatingPointRNE, unumeral.ArbitraryFloatNearestEven},
		{f.context.FloatingPointRNA, unumeral.ArbitraryFloatNearestAway},
		{f.context.FloatingPointRTP, unumeral.ArbitraryFloatTowardPositive},
		{f.context.FloatingPointRTN, unumeral.ArbitraryFloatTowardNegative},
		{f.context.FloatingPointRTZ, unumeral.ArbitraryFloatTowardZero},
	}
	for index, row := range rows {
		value, err := row.make()
		value = f.pin(t, value, err)
		found, got, err := f.manager.ReadFPARoundingMode(value)
		if err != nil || !found || got != row.want {
			t.Fatal("wrong rounding mode", index, found, got, err)
		}
	}
}
