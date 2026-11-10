package interp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestInterpolationManagerSimplifiersRecoverAfterAllocationFailure(t *testing.T) {
	for _, strong := range []bool{false, true} {
		t.Run(fmt.Sprint("strong=", strong), func(t *testing.T) {
			complete, recoveredSeen := false, false
			for ordinal := int64(0); ordinal < 128 && !complete; ordinal++ {
				t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
					m, w := interpRewritingManagerFixture(t)
					sort, err := w.IntType()
					interpMust(t, err)
					x, err := w.MakeVariable("recovery-x", sort)
					interpMust(t, err)
					x = interpPin(t, m, x)
					zero, err := w.MakeIntText("0")
					interpMust(t, err)
					term := x
					for i := 0; i < 256; i++ {
						term, err = w.Make(OperatorPlus, term, zero)
						interpMust(t, err)
					}
					term = interpPin(t, m, term)
					rewrite := w.Simplify
					other := w.ReallySimplify
					if strong {
						rewrite, other = w.ReallySimplify, w.Simplify
					}
					m.Budget().FailAfter(ordinal)
					out, failure := rewrite(term)
					m.Budget().FailAfter(-1)
					interrupted := false
					if failure != nil {
						if !errors.Is(failure, arena.ErrInjectedFailure) {
							t.Fatal("unexpected rewrite failure", failure)
						}
						if out != (ast.ASTRef{}) {
							t.Fatal("failed rewrite published an owner")
						}
						interrupted = true
					} else {
						complete = true
						interpMust(t, m.DelRef(&out))
					}
					for _, retry := range []func(ast.ASTHandle) (ast.ASTRef, error){other, rewrite} {
						result, err := retry(term)
						interpMust(t, err)
						got, err := m.RefGet(result)
						interpMust(t, err)
						if got != x {
							t.Fatal("recovery changed simplification", got, x)
						}
						interpMust(t, m.DelRef(&result))
					}
					recoveredSeen = recoveredSeen || interrupted
				})
			}
			// Each call builds its own th_rewriter (z3_simplify's local), so an
			// interrupted call leaves no state behind: the retries above are the
			// recovery check. Requiring the injected call itself to finish within
			// this fixed prefix count makes the test depend on the build mode's
			// AST allocation strategy; g3debug deliberately needs more reservations.
			if !recoveredSeen {
				t.Fatal("allocation campaign did not cover interrupted traversal and recovery", complete, recoveredSeen)
			}
		})
	}
}
