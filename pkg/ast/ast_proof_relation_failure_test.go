//go:build g3trace

package ast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Relation and symmetry constructors first create a fact and then allocate its
// proof. Failure of the latter must release the temporary fact and its inputs.
func TestASTProofRelationAndSymmetryFailureReleaseFact(t *testing.T) {
	for _, symmetry := range []bool{false, true} {
		t.Run(fmt.Sprintf("symmetry=%v", symmetry), func(t *testing.T) {
			complete, failures := false, 0
			for allowance := int64(0); allowance < 64 && !complete; allowance++ {
				t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					m := astTestManager(t)
					astTestMust(t, m.ToggleProofMode(ASTProofEnabled))
					a, b := proofTestBool(t, m, "relation-a"), proofTestBool(t, m, "relation-b")
					astTestPin(t, m, a)
					astTestPin(t, m, b)
					parent, err := m.MkProofDER(a, b)
					astTestMust(t, err)
					parentRef, err := m.NewRef(parent)
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, m.DelRef(&parentRef)) })
					construct := func() (ASTHandle, error) {
						if symmetry {
							return m.MkProofSymmetry(parent)
						}
						return m.MkProofDER(b, a)
					}
					// Warm the persistent proof declarations, not the fresh fact.
					warm, err := construct()
					astTestMust(t, err)
					warmRef, err := m.NewRef(warm)
					astTestMust(t, err)
					astTestMust(t, m.DelRef(&warmRef))
					before := m.Stats()
					m.Budget().FailAfter(allowance)
					proof, cause := construct()
					m.Budget().FailAfter(-1)
					if cause != nil {
						failures++
						if !errors.Is(cause, arena.ErrInjectedFailure) || !proof.IsNull() {
							t.Fatal("unexpected failure or proof", cause, proof)
						}
					} else {
						complete = true
						ref, err := m.NewRef(proof)
						astTestMust(t, err)
						astTestMust(t, m.DelRef(&ref))
					}
					after := m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
						t.Fatal("temporary proof fact leaked", before, after, cause)
					}
					proof, err = construct()
					astTestMust(t, err)
					ref, err := m.NewRef(proof)
					astTestMust(t, err)
					astTestMust(t, m.DelRef(&ref))
					if got := m.Stats(); got.LiveNodes != before.LiveNodes || got.LiveReferences != before.LiveReferences {
						t.Fatal("retry leaked temporary proof fact", before, got)
					}
				})
				if t.Failed() {
					return
				}
			}
			if failures == 0 || !complete {
				t.Fatal("missing failure or success", failures, complete)
			}
		})
	}
}
