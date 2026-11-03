package ast

// Source: ast_manager::mk_app and basic_decl_plugin::mk_func_decl for
// parameterized proof applications. Their fresh declaration must remain
// owned across application allocation, including a refused allocation.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTParameterizedProofApplicationFailureReleasesFreshDeclaration(t *testing.T) {
	for _, premiseCount := range []int{2, 3} {
		t.Run(fmt.Sprint(premiseCount, "-premises"), func(t *testing.T) {
			m := astTestManager(t)
			astTestMust(t, m.ToggleProofMode(ASTProofEnabled))
			conclusion := proofTestBool(t, m, "proof-rollback-goal")
			astTestPin(t, m, conclusion)
			body := make([]ASTHandle, premiseCount-1)
			for i := range body {
				body[i] = proofTestBool(t, m, fmt.Sprint("proof-rollback-premise-", i))
				astTestPin(t, m, body[i])
			}
			antecedent := body[0]
			if len(body) > 1 {
				var err error
				antecedent, err = m.MkAndArgs(body)
				astTestMust(t, err)
				astTestPin(t, m, antecedent)
			}
			clause, err := m.MkImplies(antecedent, conclusion)
			astTestMust(t, err)
			astTestPin(t, m, clause)
			premises := make([]ASTHandle, premiseCount)
			for i, fact := range append([]ASTHandle{clause}, body...) {
				premises[i], err = m.MkProofAsserted(fact)
				astTestMust(t, err)
				astTestPin(t, m, premises[i])
			}
			positions := make([]ASTProofPosition, len(body))
			for i := range positions {
				positions[i] = ASTProofPosition{First: uint32(i + 1), Second: 0}
			}
			substitutions := make([][]ASTHandle, premiseCount)
			construct := func() (ASTHandle, error) {
				return m.MkProofHyperResolve(premises, conclusion, positions, substitutions)
			}
			release := func(proof ASTHandle) {
				t.Helper()
				ref, err := m.NewRef(proof)
				astTestMust(t, err)
				proofTestFact(t, m, proof, conclusion)
				astTestMust(t, m.DelRef(&ref))
			}
			// Warm declaration spelling, slab pages and temporary ownership
			// slots, then retire the application and its uncached declaration.
			warm, err := construct()
			astTestMust(t, err)
			release(warm)
			before, bytesBefore := m.Stats(), m.Budget().Used()
			complete, failures := false, 0
			for stop := int64(0); stop < 128 && !complete; stop++ {
				m.Budget().FailAfter(stop)
				proof, cause := construct()
				m.Budget().FailAfter(-1)
				if cause != nil {
					failures++
					if !errors.Is(cause, arena.ErrInjectedFailure) || !proof.IsNull() {
						t.Fatal("failed proof application published an output or unexpected error", stop, proof, cause)
					}
				} else {
					complete = true
					release(proof)
				}
				after := m.Stats()
				if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters || m.Budget().Used() != bytesBefore {
					t.Fatalf("proof frontier %d left declaration, domain or owner storage: nodes %d/%d refs %d/%d infos %d/%d parameters %d/%d bytes %d/%d; cause %v", stop, after.LiveNodes, before.LiveNodes, after.LiveReferences, before.LiveReferences, after.LiveInfos, before.LiveInfos, after.LiveParameters, before.LiveParameters, m.Budget().Used(), bytesBefore, cause)
				}
			}
			if !complete || ASTSourceDebug && failures == 0 {
				t.Fatal("proof allocation campaign did not reach expected boundaries", complete, failures)
			}
		})
	}
}
