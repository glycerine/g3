package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source dl_base.cpp table_base::to_formula owns each tuple's equalities and
// the final disjunction while constructing the answer to the published query.
func TestExpressionAnswerAllocationFailureKeepsPublishedQuery(t *testing.T) {
	failures, completed := 0, false
	successfulAllowance := int64(-1)
	// Debug AST storage uses direct blocks, so the 512 fresh row equalities
	// need more reservations than the pooled native allocator. Exercise every
	// allowance through completion in both profiles, including the original 0–255.
	for allowance := int64(0); allowance <= 1024 && !completed; allowance++ {
		if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newQueryVariablesOwnershipFixture(t)
			fragmentMust(t, f.e.SelectEngine("datalog"))
			bits := f.own(f.m.BitVectorSort(10))
			x := f.own(f.m.MkVar(2, bits))
			predicate, query := f.predicate("answer-owner-probe-P", x)
			for value := uint64(0); value < 512; value++ {
				numeral := f.own(f.m.BitVectorNumeralUint64(value, 10))
				fragmentMust(t, f.e.AddFact(predicate, []ast.ASTHandle{numeral}))
			}
			status, err := f.e.QueryExpression(query)
			fragmentMust(t, err)
			if status != lbool.True || f.e.queryClone == nil || !f.e.queryReady {
				t.Fatal("public query did not publish a successful clone", status)
			}
			clone, head, revision := f.e.queryClone, f.e.queryClonePredicate, f.e.queryCloneRevision

			// Source ast.cpp cachedEquality keeps one declaration reference per
			// sort. Warm that legitimate cache without warming answer nodes: the
			// original query uses #2, whereas its answer uses compacted #0.
			zero, err := f.m.BitVectorNumeralUint64(0, 10)
			fragmentMust(t, err)
			warm, err := f.m.MkEq(x, zero)
			fragmentMust(t, err)
			warmRef, err := f.m.NewRef(warm)
			fragmentMust(t, err)
			equalityDecl, err := f.m.GetDecl(warm)
			fragmentMust(t, err)
			fragmentMust(t, f.m.DelRef(&warmRef))

			// Ordinary caller roots align the next reference-page allocation
			// with answer construction, exposing a fresh equality's late pin.
			for i := 0; i < 3; i++ {
				ref, err := f.m.NewRef(ast.ASTHandle{})
				fragmentMust(t, err)
				f.roots = append(f.roots, ref)
			}
			before := contextDisplayLiveCounts(f.m.Stats())
			checkUnchanged := func() {
				t.Helper()
				if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
					t.Fatal("answer construction leaked AST ownership", before, after)
				}
				cacheRefs, err := f.m.GetRefCount(equalityDecl)
				fragmentMust(t, err)
				if cacheRefs != 1 {
					t.Fatal("equality declaration lost its sole cache owner", cacheRefs)
				}
				if f.e.queryClone != clone || f.e.queryClonePredicate != head || f.e.queryCloneRevision != revision || !f.e.queryReady {
					t.Fatal("answer construction changed the published query snapshot")
				}
			}
			checkUnchanged()
			f.m.Budget().FailAfter(allowance)
			answer, cause := f.e.ExpressionAnswer()
			f.m.Budget().FailAfter(-1)
			if cause != nil {
				failures++
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("answer construction lost the allocation failure", cause)
				}
				if answer != (ast.ASTRef{}) {
					fragmentMust(t, f.m.DelRef(&answer))
					t.Fatal("failed answer construction published an output")
				}
			} else {
				completed = true
				successfulAllowance = allowance
				fragmentMust(t, f.m.DelRef(&answer))
			}
			checkUnchanged()

			// Release the retry before rewriting it: semantic checks can warm
			// unrelated manager caches and must not conceal an ownership drift.
			answer, err = f.e.ExpressionAnswer()
			fragmentMust(t, err)
			fragmentMust(t, f.m.DelRef(&answer))
			checkUnchanged()
			if !completed {
				return
			}

			answer, err = f.e.ExpressionAnswer()
			fragmentMust(t, err)
			f.roots = append(f.roots, answer)
			formula, err := f.m.RefGet(answer)
			fragmentMust(t, err)
			absentValue := f.own(f.m.BitVectorNumeralUint64(512, 10))
			absentQuery := f.own(f.m.MkApp(predicate, []ast.ASTHandle{absentValue}))
			status, err = f.e.QueryExpression(absentQuery)
			fragmentMust(t, err)
			if status != lbool.False || f.e.queryClone == clone {
				t.Fatal("absent tuple query did not replace the previous clone", status)
			}
			if held, err := f.m.RefGet(answer); err != nil || held != formula {
				t.Fatal("held answer changed after disposing its query clone", held, err)
			}
			reduced, err := f.m.NewRef(ast.ASTHandle{})
			fragmentMust(t, err)
			f.roots = append(f.roots, reduced)
			for _, value := range []uint64{0, 511, 512, 1023} {
				numeral := f.own(f.m.BitVectorNumeralUint64(value, 10))
				fragmentMust(t, f.e.guard.RewriteWithBindings(formula, []ast.ASTHandle{numeral}, reduced))
				result, err := f.m.RefGet(reduced)
				fragmentMust(t, err)
				if f.m.IsTrue(result) != (value < 512) || !f.m.IsTrue(result) && !f.m.IsFalse(result) {
					t.Fatal("held answer changed the finite relation", value)
				}
			}
			fragmentMust(t, f.e.Close())
			if held, err := f.m.RefGet(answer); err != nil || held != formula {
				t.Fatal("held answer changed after engine disposal", held, err)
			}
		}) {
			break
		}
	}
	if failures == 0 || !completed {
		t.Fatal("missing answer allocation failure/completion frontier", failures, completed)
	}
	t.Log("successful tested allocation allowance", successfulAllowance, "injected failures", failures)
}
