package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointNumericSortAliasShapes(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	rows := []struct {
		make  func() (ast.ASTHandle, error)
		ebits uint32
		sbits uint32
	}{
		{f.context.FloatingPointSort16, 5, 11},
		{f.context.FloatingPointSort32, 8, 24},
		{f.context.FloatingPointSort64, 11, 53},
		{f.context.FloatingPointSort128, 15, 113},
	}
	for index, row := range rows {
		sort, err := row.make()
		sort = f.pin(t, sort, err)
		ebits, sbits, err := f.context.FloatingPointSortShape(sort)
		if err != nil || ebits != row.ebits || sbits != row.sbits {
			t.Fatal("wrong numeric sort shape", index, ebits, sbits, err)
		}
	}
}
