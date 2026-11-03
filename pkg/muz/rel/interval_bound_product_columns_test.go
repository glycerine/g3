package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalBoundProductFullAndDuplicateColumnAssembly(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	kinds := []string{"bound_relation", "interval_relation"}
	product, selected, err := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer}, kinds, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("exact product pair was not selected")
	}
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	astTestMust(t, product.AddFactExpressions(m, []ast.ASTHandle{integer}, []ast.ASTHandle{two}))

	appended, err := product.AppendFullColumn(m, []ast.ASTHandle{integer}, real)
	astTestMust(t, err)
	if got := appended.Kinds(); len(got) != 2 || got[0] != kinds[0] || got[1] != kinds[1] {
		t.Fatal("full-column join changed requested component order", got)
	}
	if !appended.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(99, 1)}) {
		t.Fatal("appended compiler column was not full")
	}
	if !product.ContainsFact([]*big.Rat{big.NewRat(2, 1)}) || product.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(99, 1)}) {
		t.Fatal("full-column assembly mutated the source product")
	}

	duplicate, err := product.DuplicateColumn(m, []ast.ASTHandle{integer}, 0)
	astTestMust(t, err)
	if !duplicate.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(2, 1)}) || duplicate.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}) {
		t.Fatal("repeated-head column equality was not enforced in both components")
	}
	if _, err = product.AppendFullColumn(m, []ast.ASTHandle{integer}, boolean); err == nil {
		t.Fatal("unsupported full Boolean product column was accepted")
	}
	if _, err = product.DuplicateColumn(m, []ast.ASTHandle{integer}, 1); err == nil {
		t.Fatal("out-of-range duplicate source column was accepted")
	}
	if !product.ContainsFact([]*big.Rat{big.NewRat(2, 1)}) {
		t.Fatal("rejected column assembly mutated the source product")
	}
}

func TestIntervalBoundProductNullaryFullColumnAndEmptyPropagation(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	kinds := []string{"interval_relation", "bound_relation"}
	unit, selected, err := NewSelectedIntervalBoundProduct(m, nil, kinds, false)
	astTestMust(t, err)
	if !selected {
		t.Fatal("nullary exact product pair was not selected")
	}
	full, err := unit.AppendFullColumn(m, nil, integer)
	astTestMust(t, err)
	if !full.ContainsFact([]*big.Rat{big.NewRat(-100, 1)}) {
		t.Fatal("nullary identity did not produce full unary product")
	}
	empty, selected, err := NewSelectedIntervalBoundProduct(m, nil, kinds, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("empty nullary exact product pair was not selected")
	}
	emptyColumn, err := empty.AppendFullColumn(m, nil, integer)
	astTestMust(t, err)
	if !emptyColumn.Empty() {
		t.Fatal("full-column join did not preserve empty working product")
	}
}
