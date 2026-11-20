package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTAuxiliaryClauseNullAntecedentPreservesAssumptions(t *testing.T) {
	for _, fullConflict := range []bool{false, true} {
		t.Run(fmt.Sprint(fullConflict), func(t *testing.T) {
			m := astTestManager(t)
			_, _, state := smtInternalizerFixture(t, m, nil)
			a := smtRelevancyBool(t, m, "aux-core-a")
			b := smtRelevancyBool(t, m, "aux-core-b")
			for _, h := range []ast.ASTHandle{a, b} {
				astTestMust(t, state.Internalize(h, true))
			}
			la, err := state.Literal(a)
			astTestMust(t, err)
			lb, err := state.Literal(b)
			astTestMust(t, err)
			astTestMust(t, state.PushScope())
			astTestMust(t, state.AssignLiteral(la.Negated(), NewSMTBooleanAxiom()))
			state.booleanData[la.Var()].SetAssumption(true)
			want := 1
			if fullConflict {
				astTestMust(t, state.AssignLiteral(lb.Negated(), NewSMTBooleanAxiom()))
				state.booleanData[lb.Var()].SetAssumption(true)
				want = 2
			}
			astTestMust(t, state.addClause([]SMTLiteral{la, lb}, false, ast.ASTHandle{}))
			if !fullConflict {
				astTestMust(t, state.AssignLiteral(lb.Negated(), NewSMTBooleanAxiom()))
			}
			conflict, ok := state.Conflict()
			if !ok {
				t.Fatal("missing auxiliary clause conflict")
			}
			resolver, err := NewSMTConflictResolver(state)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, resolver.Close()) })
			core, err := resolver.UnsatCore(conflict, state.ConflictLiteral())
			astTestMust(t, err)
			seen := make(map[SMTLiteral]bool)
			for _, literal := range core {
				seen[literal] = true
			}
			// Source mk_unsat_core repeats raw assumption discoveries;
			// context::mk_unsat_core deduplicates them before publication.
			if len(seen) != want || !seen[la.Negated()] || fullConflict && !seen[lb.Negated()] {
				t.Fatal("lost simplifying assumptions", core)
			}
		})
	}
}
