package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointNumericSortAliasesCanonical(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	rows := []struct {
		numeric func() (ast.ASTHandle, error)
		named   func() (ast.ASTHandle, error)
	}{
		{f.context.FloatingPointSort16, f.context.FloatingPointHalfSort},
		{f.context.FloatingPointSort32, f.context.FloatingPointSingleSort},
		{f.context.FloatingPointSort64, f.context.FloatingPointDoubleSort},
		{f.context.FloatingPointSort128, f.context.FloatingPointQuadrupleSort},
	}
	for index, row := range rows {
		numeric, err := row.numeric()
		numeric = f.pin(t, numeric, err)
		named, err := row.named()
		named = f.pin(t, named, err)
		if numeric != named {
			t.Fatal("noncanonical sort alias", index, numeric, named)
		}
	}
}
