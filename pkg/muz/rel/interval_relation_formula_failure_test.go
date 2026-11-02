package rel

// Source ownership audit for interval_relation::to_formula.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationToFormulaAllocationFailureAndRetry(t *testing.T) {
	firstSuccess := -1
	for stop := int64(0); stop < 128; stop++ {
		succeeded := false
		t.Run("allocation", func(t *testing.T) {
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			real, err := m.MkFamilySort(family, 0, nil)
			astTestMust(t, err)
			astTestPin(t, m, real)
			own := func(h ast.ASTHandle, e error) ast.ASTHandle {
				astTestMust(t, e)
				astTestPin(t, m, h)
				return h
			}
			x := own(m.MkVar(0, real))
			zero, two := own(m.ArithmeticRealNumeralText("0")), own(m.ArithmeticRealNumeralText("2"))
			r := NewIntervalFull(2)
			astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticLT(zero, x))))
			astTestMust(t, r.FilterInterpreted(m, own(m.MkArithmeticLE(x, two))))
			astTestMust(t, r.FilterIdenticalColumns([]uint32{0, 1}))

			before := m.Stats().LiveReferences
			m.Budget().FailAfter(stop)
			formula, cause := r.ToFormula(m, []ast.ASTHandle{real, real})
			m.Budget().FailAfter(-1)
			if cause != nil {
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("unexpected construction failure", cause)
				}
				if formula != (ast.ASTRef{}) {
					t.Fatal("failed formula construction published a reference")
				}
				if after := m.Stats().LiveReferences; after != before {
					t.Fatalf("failed formula construction changed live references: %d -> %d", before, after)
				}
				return
			}
			succeeded = true
			firstSuccess = int(stop)
			if formula == (ast.ASTRef{}) || m.Stats().LiveReferences != before+1 {
				t.Fatal("successful formula publication has wrong ownership")
			}
			h, err := m.RefGet(formula)
			astTestMust(t, err)
			if kind, err := m.GetDeclKind(h); err != nil || kind != ast.ASTOpAnd {
				t.Fatal("successful retry did not publish the bounded equality formula", kind, err)
			}
			astTestMust(t, m.DelRef(&formula))
			if after := m.Stats().LiveReferences; after != before {
				t.Fatalf("released formula changed live references: %d -> %d", before, after)
			}
		})
		if succeeded {
			break
		}
	}
	if firstSuccess <= 0 {
		t.Fatal("allocation campaign did not cross a failure boundary", firstSuccess)
	}
}
