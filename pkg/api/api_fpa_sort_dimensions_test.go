package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointStandardSortDimensions(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	rows := []struct {
		make        func() (ast.ASTHandle, error)
		exponent    uint32
		significand uint32
	}{
		{f.context.FloatingPointHalfSort, 5, 11},
		{f.context.FloatingPointSingleSort, 8, 24},
		{f.context.FloatingPointDoubleSort, 11, 53},
		{f.context.FloatingPointQuadrupleSort, 15, 113},
	}
	for index, row := range rows {
		sort, err := row.make()
		sort = f.pin(t, sort, err)
		exponent, exponentErr := f.context.FloatingPointExponentBits(sort)
		significand, significandErr := f.context.FloatingPointSignificandBits(sort)
		if exponentErr != nil || significandErr != nil ||
			exponent != row.exponent || significand != row.significand {
			t.Fatal("wrong standard sort dimensions", index,
				exponent, significand, exponentErr, significandErr)
		}
	}
}
