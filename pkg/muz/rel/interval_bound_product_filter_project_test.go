package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalBoundProductFilterInterpretedAndProject(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	kinds := []string{"bound_relation", "interval_relation"}
	product, selected, err := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer, integer}, kinds, false)
	astTestMust(t, err)
	if !selected {
		t.Fatal("exact product pair was not selected")
	}
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	atMostThree, err := m.MkArithmeticLE(x, three)
	astTestMust(t, err)
	projected, err := product.FilterInterpretedAndProject(m, atMostThree, []uint32{1})
	astTestMust(t, err)
	if !projected.ContainsFact([]*big.Rat{big.NewRat(3, 1)}) || projected.ContainsFact([]*big.Rat{big.NewRat(4, 1)}) {
		t.Fatal("fused interpreted filter was not applied before projection")
	}
	if !product.ContainsFact([]*big.Rat{big.NewRat(4, 1), big.NewRat(0, 1)}) {
		t.Fatal("fused filter/project mutated its source")
	}
	if got := projected.Kinds(); len(got) != 2 || got[0] != kinds[0] || got[1] != kinds[1] {
		t.Fatal("fused filter/project changed requested component order", got)
	}
}

func TestIntervalBoundProductFilterProjectConjunctionAndFailureSemantics(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	product, selected, err := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer, integer}, []string{"interval_relation", "bound_relation"}, false)
	astTestMust(t, err)
	if !selected {
		t.Fatal("exact product pair was not selected")
	}
	x, err := m.MkVar(0, integer)
	astTestMust(t, err)
	one, err := m.ArithmeticIntegerNumeralText("1")
	astTestMust(t, err)
	three, err := m.ArithmeticIntegerNumeralText("3")
	astTestMust(t, err)
	lower, err := m.MkArithmeticLE(one, x)
	astTestMust(t, err)
	upper, err := m.MkArithmeticLE(x, three)
	astTestMust(t, err)
	conjunction, err := m.MkAnd(lower, upper)
	astTestMust(t, err)
	coarse, err := product.FilterInterpretedAndProject(m, conjunction, []uint32{1})
	astTestMust(t, err)
	// dl_compiler supplies one conjunction. Neither concrete component
	// recognizes that compound shape, so the source operation remains coarse.
	if !coarse.ContainsFact([]*big.Rat{big.NewRat(-100, 1)}) || !coarse.ContainsFact([]*big.Rat{big.NewRat(100, 1)}) {
		t.Fatal("compound interpreted condition was unsoundly split into stronger filters")
	}
	if _, err = product.FilterInterpretedAndProject(m, upper, []uint32{2}); err == nil {
		t.Fatal("out-of-range fused projection was accepted")
	}
	if _, err = product.FilterInterpretedAndProject(m, upper, nil); err == nil {
		t.Fatal("empty fused projection list was accepted")
	}
	if !product.ContainsFact([]*big.Rat{big.NewRat(100, 1), big.NewRat(-100, 1)}) {
		t.Fatal("rejected fused operation mutated its source")
	}
	empty, selected, err := NewSelectedIntervalBoundProduct(m, []ast.ASTHandle{integer, integer}, []string{"interval_relation", "bound_relation"}, true)
	astTestMust(t, err)
	if !selected {
		t.Fatal("empty exact product pair was not selected")
	}
	emptyProjected, err := empty.FilterInterpretedAndProject(m, upper, []uint32{1})
	astTestMust(t, err)
	if !emptyProjected.Empty() {
		t.Fatal("fused filter/project did not preserve empty input")
	}
}
