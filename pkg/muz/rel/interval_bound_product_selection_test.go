package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalBoundProductSelectionClonePrecisionAndOrder(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	orders := [][]string{{"bound_relation", "interval_relation"}, {"interval_relation", "bound_relation"}}
	for _, kinds := range orders {
		product, selected, cause := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer, real}, kinds, true)
		astTestMust(t, cause)
		if !selected || product == nil {
			t.Fatal("exact bound/interval pair was not selected", kinds)
		}
		if got := product.Kinds(); len(got) != 2 || got[0] != kinds[0] || got[1] != kinds[1] {
			t.Fatal("selected product lost component order", kinds, got)
		}
		astTestMust(t, product.AddFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}))
		clone, cause := product.Clone()
		astTestMust(t, cause)
		astTestMust(t, product.FilterEqual(0, big.NewRat(3, 1)))
		if !product.Empty() || !clone.ContainsFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}) {
			t.Fatal("clone borrowed source component mutation", kinds)
		}
		if precise, precisionErr := clone.IsPrecise(); precisionErr != nil || precise {
			t.Fatal("bound/interval product precision", precise, precisionErr)
		}
		projected, cause := clone.Project([]uint32{1})
		astTestMust(t, cause)
		if got := projected.Kinds(); got[0] != kinds[0] || got[1] != kinds[1] {
			t.Fatal("projection lost component order", got)
		}
		joined, cause := JoinIntervalBoundProducts(projected, NewIntervalBoundProduct(1, false), nil, nil)
		astTestMust(t, cause)
		if got := joined.Kinds(); got[0] != kinds[0] || got[1] != kinds[1] {
			t.Fatal("join lost left component order", got)
		}
	}

	nullary, selected, err := NewSelectedIntervalBoundProduct(m, nil, orders[0], false)
	astTestMust(t, err)
	if !selected || nullary == nil || nullary.Empty() {
		t.Fatal("nullary full product selection")
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	if product, exact, cause := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{boolean}, orders[0], true); !exact || cause == nil || product != nil {
		t.Fatal("exact pair accepted unsupported Boolean signature", product, exact, cause)
	}
	if product, exact, cause := NewSelectedIntervalBoundProduct(nil, nil, []string{"bound_relation"}, true); exact || cause != nil || product != nil {
		t.Fatal("non-pair touched manager or selected", product, exact, cause)
	}
}

func TestIntervalBoundProductASTFactInsertionIsValidatedBeforeMutation(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	product, selected, err := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer}, []string{"bound_relation", "interval_relation"}, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("exact pair not selected")
	}
	truth, err := m.MkTrue()
	astTestMust(t, err)
	if err = product.AddFactExpressions(m, []ast.ASTHandle{integer}, []ast.ASTHandle{truth}); err == nil {
		t.Fatal("non-arithmetic fact accepted")
	}
	if !product.Empty() {
		t.Fatal("rejected fact partially mutated product")
	}
	symbol, err := m.Symbols.Intern("non-numeral-int")
	astTestMust(t, err)
	declaration, err := m.MkFuncDecl(symbol, nil, integer, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	value, err := m.MkApp(declaration, nil)
	astTestMust(t, err)
	if err = product.AddFactExpressions(m, []ast.ASTHandle{integer}, []ast.ASTHandle{value}); err == nil {
		t.Fatal("symbolic arithmetic fact accepted")
	}
	if !product.Empty() {
		t.Fatal("symbolic rejection partially mutated product")
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	astTestMust(t, product.AddFactExpressions(m, []ast.ASTHandle{integer}, []ast.ASTHandle{one}))
	if product.Empty() || !product.ContainsFact([]*big.Rat{big.NewRat(1, 1)}) {
		t.Fatal("exact numeral fact was not added to both components")
	}
}
