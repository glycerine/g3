package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalBoundProductConstantColumnSourceSemantics(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	kinds := []string{"bound_relation", "interval_relation"}
	product, selected, err := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer}, kinds, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("exact product pair was not selected")
	}
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	astTestMust(t, product.AddFactExpressions(m, []ast.ASTHandle{integer}, []ast.ASTHandle{two}))
	constant, err := product.AppendConstantColumn(m, []ast.ASTHandle{integer}, integer, seven)
	astTestMust(t, err)
	if !constant.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(7, 1)}) || constant.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(8, 1)}) {
		t.Fatal("constant compiler column did not constrain the interval component")
	}
	if !constant.bound.ContainsFact(make([]ast.ASTHandle, 2)) {
		t.Fatal("constant compiler column unexpectedly constrained coarse bound component")
	}
	if got := constant.Kinds(); len(got) != 2 || got[0] != kinds[0] || got[1] != kinds[1] {
		t.Fatal("constant join changed requested component order", got)
	}
	if !product.ContainsFact([]*big.Rat{big.NewRat(2, 1)}) {
		t.Fatal("constant assembly mutated source product")
	}
}

func TestIntervalBoundProductConstantColumnNullaryAndRejection(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	kinds := []string{"interval_relation", "bound_relation"}
	unit, selected, err := NewSelectedIntervalBoundProduct(m, nil, kinds, false)
	astTestMust(t, err)
	if !selected {
		t.Fatal("nullary exact product pair was not selected")
	}
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	constant, err := unit.AppendConstantColumn(m, nil, integer, three)
	astTestMust(t, err)
	if !constant.ContainsFact([]*big.Rat{big.NewRat(3, 1)}) || constant.ContainsFact([]*big.Rat{big.NewRat(4, 1)}) {
		t.Fatal("nullary constant did not construct a unary singleton product")
	}
	if _, err = unit.AppendConstantColumn(m, nil, real, three); err == nil {
		t.Fatal("ill-sorted constant column was accepted")
	}
	name, err := m.Symbols.Intern("bound-interval-symbolic-constant")
	astTestMust(t, err)
	declaration, err := m.MkFuncDecl(name, nil, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	symbolic, err := m.MkApp(declaration, nil)
	astTestMust(t, err)
	if _, err = unit.AppendConstantColumn(m, nil, integer, symbolic); err == nil {
		t.Fatal("symbolic arithmetic constant column was accepted")
	}
	if unit.Empty() {
		t.Fatal("rejected constant assembly mutated nullary source")
	}
}
