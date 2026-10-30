package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Source theory_arith_core.h::propagate_core applies newly asserted bounds
// to the retained tableau; it does not reconstruct every unrelated row.
func TestSMTArithmeticRetainsTableauAcrossBoundAssignments(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTLinearArithmeticTheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "incremental-x"), sort)
	astTestMust(t, err)
	one, err := m.ArithmeticRealNumeralText("1")
	astTestMust(t, err)
	bound, err := m.MkFamilyApp(family, smtArithOpGE, nil, []ast.ASTHandle{x, one})
	astTestMust(t, err)
	astTestMust(t, state.Internalize(bound, true))
	astTestMust(t, theory.Propagate())
	original := theory.solver
	astTestMust(t, state.PushScope())
	literal, err := state.Literal(bound)
	astTestMust(t, err)
	astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
	astTestMust(t, state.PropagateTheoryAssignments())
	astTestMust(t, theory.Propagate())
	if theory.solver != original {
		t.Fatal("assigning a bound rebuilt the unchanged arithmetic tableau")
	}
	column := theory.atoms[theory.atomByBool[literal.Var()]].form.terms
	for v := range column {
		if lo, ok := theory.solver.Lower(v); !ok || lo.Main.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal("new bound not installed", lo, ok)
		}
	}
	astTestMust(t, state.PopScopes(1))
	for v := range column {
		if _, ok := theory.solver.Lower(v); ok {
			t.Fatal("popped bound survived tableau rollback")
		}
	}
}
