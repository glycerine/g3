package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationHeadAssemblyColumns(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)

	source := NewIntervalEmpty(1)
	astTestMust(t, source.AddFact([]*big.Rat{big.NewRat(2, 1)}))
	astTestMust(t, source.AddFact([]*big.Rat{big.NewRat(4, 1)}))
	full, err := source.AppendFullColumn(m, []ast.ASTHandle{integer}, real)
	astTestMust(t, err)
	if !full.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(99, 1)}) {
		t.Fatal("unbound head column was not full")
	}
	duplicate, err := source.DuplicateColumn(m, []ast.ASTHandle{integer}, 0)
	astTestMust(t, err)
	if !duplicate.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(2, 1)}) || duplicate.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(4, 1)}) {
		t.Fatal("repeated head column did not preserve its assembly equality")
	}
	seven, err := m.ArithmeticIntegerNumeralText("7")
	astTestMust(t, err)
	constant, err := source.AppendConstantColumn(m, []ast.ASTHandle{integer}, integer, seven)
	astTestMust(t, err)
	if !constant.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(7, 1)}) || constant.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(8, 1)}) {
		t.Fatal("constant head column was not a unary singleton")
	}
	if !source.ContainsFact([]*big.Rat{big.NewRat(3, 1)}) {
		t.Fatal("column assembly mutated its source")
	}
	if _, err = source.AppendFullColumn(m, []ast.ASTHandle{integer}, boolean); err == nil {
		t.Fatal("finite full column was accepted by interval assembly")
	}
	if _, err = source.DuplicateColumn(m, []ast.ASTHandle{integer}, 1); err == nil {
		t.Fatal("out-of-range duplicate column was accepted")
	}

	unit := NewIntervalFull(0)
	nullaryFull, err := unit.AppendFullColumn(m, nil, integer)
	astTestMust(t, err)
	if !nullaryFull.ContainsFact([]*big.Rat{big.NewRat(-100, 1)}) {
		t.Fatal("nullary identity did not produce a full unary relation")
	}
	nullaryConstant, err := unit.AppendConstantColumn(m, nil, integer, seven)
	astTestMust(t, err)
	if !nullaryConstant.ContainsFact([]*big.Rat{big.NewRat(7, 1)}) || nullaryConstant.ContainsFact([]*big.Rat{big.NewRat(6, 1)}) {
		t.Fatal("nullary constant did not produce a unary singleton")
	}
	empty, err := NewIntervalEmpty(0).AppendFullColumn(m, nil, integer)
	astTestMust(t, err)
	if !empty.Empty() {
		t.Fatal("full-column join did not preserve an empty working relation")
	}
}
