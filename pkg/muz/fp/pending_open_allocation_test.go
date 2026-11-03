package fp

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func newPendingOpenAllocationFixture(t *testing.T) *contextDisplayCleanupFixture {
	t.Helper()
	f := newContextDisplayCleanupFixture(t)
	p, r := f.e.predicateOrder[0], f.e.predicateOrder[2]
	boolean, err := f.m.GetDomain(p, 0)
	fragmentMust(t, err)
	x := f.own(f.m.MkVar(0, boolean))
	y := f.own(f.m.MkVar(1, boolean))
	px := f.own(f.m.MkApp(p, []ast.ASTHandle{x}))
	py := f.own(f.m.MkApp(p, []ast.ASTHandle{y}))
	head := f.own(f.m.MkApp(r, []ast.ASTHandle{x}))
	body := f.own(f.m.MkAnd(px, py))
	formula := f.own(f.m.MkImplies(body, head))
	fragmentMust(t, f.e.AddRule(formula, "fresh-open-allocation"))
	return f
}

func pendingOpenAllocationRead(t *testing.T, f *contextDisplayCleanupFixture) {
	t.Helper()
	formulas, err := f.e.RuleFormulas()
	fragmentMust(t, err)
	if len(formulas) != 4 {
		t.Fatal("pending normalization retry changed the formula sequence", len(formulas))
	}
	fragmentMust(t, closeASTRefs(f.m, formulas))
}

func TestPendingOpenRuleFormulasAllocationFailureRetryAndClose(t *testing.T) {
	var expected [4]uint64
	t.Run("uninterrupted", func(t *testing.T) {
		f := newPendingOpenAllocationFixture(t)
		pendingOpenAllocationRead(t, f)
		pendingOpenAllocationRead(t, f)
		expected = contextDisplayLiveCounts(f.m.Stats())
	})
	if t.Failed() {
		return
	}
	failures, completed := 0, 0
	for allowance := int64(0); allowance <= 2048; allowance++ {
		t.Run(fmt.Sprintf("reservations-%d", allowance), func(t *testing.T) {
			f := newPendingOpenAllocationFixture(t)
			f.m.Budget().FailAfter(allowance)
			formulas, cause := f.e.RuleFormulas()
			f.m.Budget().FailAfter(-1)
			if cause != nil {
				failures++
				var native memory.Failure
				if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &native) || native.Kind != memory.FailureMemout) {
					t.Fatal("pending normalization replaced the allocation cause", allowance, cause)
				}
				if formulas != nil {
					_ = closeASTRefs(f.m, formulas)
					t.Fatal("failed pending normalization published partial formulas")
				}
				var cleanup memory.ConstructionFailure
				if errors.As(cause, &cleanup) {
					fragmentMust(t, cleanup.Cleanup())
					fragmentMust(t, cleanup.Cleanup())
				}
			} else {
				completed++
				if len(formulas) != 4 {
					t.Fatal("completed pending normalization changed formula count", len(formulas))
				}
				fragmentMust(t, closeASTRefs(f.m, formulas))
			}
			pendingOpenAllocationRead(t, f)
			pendingOpenAllocationRead(t, f)
			if actual := contextDisplayLiveCounts(f.m.Stats()); actual != expected {
				t.Fatal("pending normalization allocation retry leaked AST ownership", allowance, expected, actual, cause)
			}
			fragmentMust(t, f.e.Close())
			fragmentMust(t, f.e.Close())
		})
		if t.Failed() || completed != 0 {
			break
		}
	}
	if failures == 0 || completed == 0 {
		t.Fatal("pending normalization campaign missed failure or completion", failures, completed)
	}
}
