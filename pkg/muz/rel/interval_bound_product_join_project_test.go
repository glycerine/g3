package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalBoundProductJoinProjectCompilerComposition(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	sorts := []ast.ASTHandle{integer, integer}
	left, selected, err := NewSelectedIntervalBoundProduct(m, sorts, []string{"bound_relation", "interval_relation"}, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("left exact product was not selected")
	}
	right, selected, err := NewSelectedIntervalBoundProduct(m, sorts, []string{"interval_relation", "bound_relation"}, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("right exact product was not selected")
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	two, err := m.ArithmeticIntegerNumeralText("2")
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	astTestMust(t, left.AddFactExpressions(m, sorts, []ast.ASTHandle{one, two}))
	astTestMust(t, right.AddFactExpressions(m, sorts, []ast.ASTHandle{two, three}))
	result, err := JoinProjectIntervalBoundProducts(left, right, []uint32{1}, []uint32{0}, []uint32{1, 2})
	astTestMust(t, err)
	if !result.ContainsFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}) || result.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}) {
		t.Fatal("fused join/project did not preserve joined endpoint columns")
	}
	if got := result.Kinds(); len(got) != 2 || got[0] != "bound_relation" || got[1] != "interval_relation" {
		t.Fatal("fused join/project did not preserve left requested order", got)
	}
	if !left.ContainsFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}) || !right.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(3, 1)}) {
		t.Fatal("fused join/project mutated an operand")
	}
}

func TestIntervalBoundProductJoinProjectEmptyAndFailureAtomicity(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	sorts := []ast.ASTHandle{integer}
	full, selected, err := NewSelectedIntervalBoundProduct(m, sorts, []string{"interval_relation", "bound_relation"}, false)
	astTestMust(t, err)
	if !selected {
		t.Fatal("full exact product was not selected")
	}
	empty, selected, err := NewSelectedIntervalBoundProduct(m, sorts, []string{"interval_relation", "bound_relation"}, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("empty exact product was not selected")
	}
	result, err := JoinProjectIntervalBoundProducts(full, empty, nil, nil, []uint32{1})
	astTestMust(t, err)
	if !result.Empty() {
		t.Fatal("fused join/project did not propagate empty operand")
	}
	if _, err = JoinProjectIntervalBoundProducts(full, full, []uint32{1}, []uint32{0}, nil); err == nil {
		t.Fatal("out-of-range join mapping was accepted")
	}
	if _, err = JoinProjectIntervalBoundProducts(full, full, nil, nil, []uint32{2}); err == nil {
		t.Fatal("out-of-range joined projection was accepted")
	}
	if full.Empty() || !full.ContainsFact([]*big.Rat{big.NewRat(100, 1)}) || !empty.Empty() {
		t.Fatal("rejected fused join/project mutated an operand")
	}
}
