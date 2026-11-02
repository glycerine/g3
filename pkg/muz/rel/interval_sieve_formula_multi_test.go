package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalSieveFormulaNonadjacentMixedSortValueOracle(t *testing.T) {
	f, integer, numeral := intervalSieveFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := f.ast.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, real)
	sorts := []ast.ASTHandle{f.boolean, integer, f.boolean, real}
	r, err := NewIntervalSieveFull(f.ast, sorts)
	astTestMust(t, err)
	two := numeral("2")
	five, err := f.ast.ArithmeticRealNumeralText("5")
	astTestMust(t, err)
	astTestPin(t, f.ast, five)
	astTestMust(t, r.FilterEqual(1, two))
	astTestMust(t, r.FilterEqual(3, five))
	formula, err := r.ToFormula()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.ast.DelRef(&formula)) }()
	got, err := f.ast.RefGet(formula)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	x := own(f.ast.MkVar(1, integer))
	y := own(f.ast.MkVar(3, real))
	want := own(f.ast.MkAndArray([]ast.ASTHandle{
		own(f.ast.MkArithmeticLE(two, x)), own(f.ast.MkArithmeticLE(x, two)),
		own(f.ast.MkArithmeticLE(five, y)), own(f.ast.MkArithmeticLE(y, five)),
	}))
	if got != want {
		t.Fatal("nonadjacent selected values mapped to wrong outer variables")
	}
}
