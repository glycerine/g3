package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestNumericRelationFiltersMutateEmptyPayloadLikeSource(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, cause error) ast.ASTHandle {
		astTestMust(t, cause)
		astTestPin(t, m, h)
		return h
	}

	t.Run("bound", func(t *testing.T) {
		x, y := own(m.MkVar(0, integer)), own(m.MkVar(1, integer))
		r := NewBoundRelation(2, true)
		astTestMust(t, r.AddLess(0, 1))
		if !r.Empty() {
			t.Fatal("filter cleared the source empty flag")
		}
		strict, weak, cause := r.Bounds(0)
		astTestMust(t, cause)
		if len(strict) != 1 || strict[0] != 1 || len(weak) != 0 {
			t.Fatal("filter did not mutate the empty relation's reset payload", strict, weak)
		}
		formula, cause := r.ToFormula(m, []ast.ASTHandle{integer, integer})
		astTestMust(t, cause)
		defer func() { astTestMust(t, m.DelRef(&formula)) }()
		got, cause := m.RefGet(formula)
		astTestMust(t, cause)
		if got != own(m.MkArithmeticLT(x, y)) {
			t.Fatal("empty relation formula ignored its mutated bound payload")
		}
	})

	t.Run("interval", func(t *testing.T) {
		x := own(m.MkVar(0, real))
		three := own(m.ArithmeticRealNumeralText("3"))
		r := NewIntervalEmpty(1)
		astTestMust(t, r.FilterInterpreted(m, own(m.MkEq(x, three))))
		if !r.Empty() {
			t.Fatal("filter cleared the source empty flag")
		}
		lower, lowerOpen, upper, upperOpen, cause := r.Bounds(0)
		astTestMust(t, cause)
		if lower == nil || upper == nil || lower.Cmp(threeRat()) != 0 || upper.Cmp(threeRat()) != 0 || lowerOpen || upperOpen {
			t.Fatal("filter did not mutate the empty relation's reset payload", lower, lowerOpen, upper, upperOpen)
		}
		formula, cause := r.ToFormula(m, []ast.ASTHandle{real})
		astTestMust(t, cause)
		defer func() { astTestMust(t, m.DelRef(&formula)) }()
		got, cause := m.RefGet(formula)
		astTestMust(t, cause)
		want := own(m.MkAnd(own(m.MkArithmeticLE(three, x)), own(m.MkArithmeticLE(x, three))))
		if got != want {
			t.Fatal("empty relation formula ignored its mutated interval payload")
		}
	})
}

func threeRat() *big.Rat { return big.NewRat(3, 1) }
