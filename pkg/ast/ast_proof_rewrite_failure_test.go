package ast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Source ast_manager::mk_rewrite creates an equality fact before its proof
// application. The Go owner must release that fresh fact if proof allocation
// fails, including when the proof declaration has already been cached.
func TestASTProofRewriteFailureReleasesFreshFact(t *testing.T) {
	for _, oeq := range []bool{false, true} {
		t.Run(fmt.Sprint("oeq=", oeq), func(t *testing.T) {
			complete, failures := false, 0
			for allowance := int64(0); allowance <= 64 && !complete; allowance++ {
				if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					// Independent managers prevent an earlier failure from warming
					// the equality under test or hiding a leaked zero-reference fact.
					m := astTestManager(t)
					astTestMust(t, m.ToggleProofMode(ASTProofEnabled))
					a, b := proofTestBool(t, m, "rewrite-a"), proofTestBool(t, m, "rewrite-b")
					astTestPin(t, m, a)
					astTestPin(t, m, b)
					construct := func() (ASTHandle, error) {
						if oeq {
							return m.MkProofOEqRewrite(a, b)
						}
						return m.MkProofRewrite(a, b)
					}
					// Keep only the persistent declaration cache warm. Releasing
					// this proof also releases its equality; every attempt below
					// must construct a fresh equality fact.
					warm, err := construct()
					astTestMust(t, err)
					warmRef, err := m.NewRef(warm)
					astTestMust(t, err)
					astTestMust(t, m.DelRef(&warmRef))
					result, err := m.NewRef(ASTHandle{})
					astTestMust(t, err)
					t.Cleanup(func() { astTestMust(t, m.DelRef(&result)) })
					before := m.Stats()
					checkReleased := func(stage string) {
						t.Helper()
						after := m.Stats()
						if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
							t.Fatalf("%s leaked fresh equality ownership: nodes %d/%d refs %d/%d", stage, after.LiveNodes, before.LiveNodes, after.LiveReferences, before.LiveReferences)
						}
					}
					checkAndRelease := func(proof ASTHandle) {
						t.Helper()
						astTestMust(t, m.SetRef(result, proof))
						isRewrite, err := m.IsProofKind(proof, ASTPrRewrite)
						astTestMust(t, err)
						if !isRewrite {
							t.Fatal("constructor did not produce a rewrite proof")
						}
						fact, err := m.GetProofFact(proof)
						astTestMust(t, err)
						decl, err := m.GetDecl(fact)
						astTestMust(t, err)
						kind, err := m.GetDeclKind(decl)
						astTestMust(t, err)
						want := ASTOpIff
						if oeq {
							want = ASTOpOEq
						}
						arity, err := m.GetNumArgs(fact)
						astTestMust(t, err)
						if kind != want || arity != 2 {
							t.Fatal("rewrite fact relation", kind, want, arity)
						}
						left, err := m.GetArg(fact, 0)
						astTestMust(t, err)
						right, err := m.GetArg(fact, 1)
						astTestMust(t, err)
						if left != a || right != b {
							t.Fatal("rewrite fact changed original inputs", left, right, a, b)
						}
						astTestMust(t, m.SetRef(result, ASTHandle{}))
					}

					m.Budget().FailAfter(allowance)
					proof, cause := construct()
					m.Budget().FailAfter(-1)
					if cause != nil {
						failures++
						if !errors.Is(cause, arena.ErrInjectedFailure) || !proof.IsNull() {
							t.Fatal("failed rewrite published a proof or unexpected error", proof, cause)
						}
					} else {
						complete = true
						checkAndRelease(proof)
					}
					checkReleased("injected attempt")
					for retry := 0; retry < 2; retry++ {
						proof, err := construct()
						astTestMust(t, err)
						checkAndRelease(proof)
						checkReleased("successful retry")
					}
				}) {
					return
				}
			}
			// Warm native slabs can complete without budget growth. Debug
			// allocation checks must exercise both failures and completion.
			if !complete || ASTSourceDebug && failures == 0 {
				t.Fatal("rewrite allocation campaign missed failure or completion", failures, complete)
			}
		})
	}
}
