package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestUnitSubsumptionTacticPreservesSurvivingProofAndDependency(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		for _, cores := range []bool{false, true} {
			t.Run(fmt.Sprintf("proofs=%v/cores=%v", proofs, cores), func(t *testing.T) {
				m, config := smtModelGeneratorFixture(t)
				if proofs {
					astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
				}
				boolSort, err := m.MkBoolSort()
				astTestMust(t, err)
				a, err := m.MkConst(astTestSymbol(t, m, "subsumption-proof-a"), boolSort)
				astTestMust(t, err)
				b, err := m.MkConst(astTestSymbol(t, m, "subsumption-proof-b"), boolSort)
				astTestMust(t, err)
				weak, err := m.MkOr(a, b)
				astTestMust(t, err)
				goal, err := tactic.NewGoal(m, proofs, false, cores)
				astTestMust(t, err)
				defer func() { astTestMust(t, goal.Close()) }()
				var weakProof, aProof ast.ASTHandle
				if proofs {
					weakProof, err = m.MkProofAsserted(weak)
					astTestMust(t, err)
					aProof, err = m.MkProofAsserted(a)
					astTestMust(t, err)
				}
				var weakDep, aDep ast.ASTDependency
				if cores {
					weakDep, err = m.DependencyLeaf(weak)
					astTestMust(t, err)
					aDep, err = m.DependencyLeaf(a)
					astTestMust(t, err)
				}
				astTestMust(t, goal.AssertWithProof(weak, weakProof, weakDep))
				astTestMust(t, goal.AssertWithProof(a, aProof, aDep))
				unit, err := NewUnitSubsumptionTactic(m, nil, SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
				astTestMust(t, err)
				defer func() { astTestMust(t, unit.Close()) }()
				var result tactic.TacticResult
				defer func() { astTestMust(t, result.Close()) }()
				astTestMust(t, unit.Apply(goal, &result))
				n, err := goal.Size()
				astTestMust(t, err)
				if n != 1 {
					t.Fatal("goal size", n)
				}
				form, err := goal.Form(0)
				astTestMust(t, err)
				if form != a {
					t.Fatal("surviving assertion", form, a)
				}
				if proofs {
					pr, err := goal.Proof(0)
					astTestMust(t, err)
					fact, err := m.GetProofFact(pr)
					astTestMust(t, err)
					if fact != a {
						t.Fatal("surviving proof fact", fact)
					}
				}
				if cores {
					dep, err := goal.Dependency(0)
					astTestMust(t, err)
					if dep != aDep {
						t.Fatal("surviving dependency", dep, aDep)
					}
				}
			})
		}
	}
}
