package fp

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type pendingFlushArchiveEntry struct {
	formula ast.ASTHandle
	name    symbol.Symbol
	bound   uint32
}

func pendingFlushArchive(t *testing.T, f *contextDisplayCleanupFixture) ([]pendingFlushArchiveEntry, []bool) {
	t.Helper()
	archive := make([]pendingFlushArchiveEntry, len(f.e.rawRules))
	phases := make([]bool, len(archive))
	for i, raw := range f.e.rawRules {
		formula, err := f.m.RefGet(raw.formula)
		fragmentMust(t, err)
		archive[i] = pendingFlushArchiveEntry{formula, raw.name, raw.bound}
		phases[i] = raw.pending
	}
	return archive, phases
}

func newPendingFlushAllocationFixture(t *testing.T) *contextDisplayCleanupFixture {
	t.Helper()
	f := newContextDisplayCleanupFixture(t)
	p, r := f.e.predicateOrder[0], f.e.predicateOrder[2]
	boolean, err := f.m.GetDomain(p, 0)
	fragmentMust(t, err)
	x, y := f.own(f.m.MkVar(0, boolean)), f.own(f.m.MkVar(1, boolean))
	px := f.own(f.m.MkApp(p, []ast.ASTHandle{x}))
	py := f.own(f.m.MkApp(p, []ast.ASTHandle{y}))
	head := f.own(f.m.MkApp(r, []ast.ASTHandle{x}))
	body := f.own(f.m.MkOr(px, py))
	implication := f.own(f.m.MkImplies(body, head))
	formula := f.own(f.m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean, boolean},
		Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)}, Body: implication,
	}))
	fragmentMust(t, f.e.AddRuleWithBound(formula, "closed-forall-split", 17))
	if free, err := f.m.HasFreeVars(formula); err != nil || free {
		t.Fatal("pending flush fixture is not an explicitly closed formula", free, err)
	}
	count, err := f.e.set.Len()
	fragmentMust(t, err)
	if count != 5 {
		t.Fatal("closed formula did not produce the expected two eager clauses", count)
	}
	return f
}

func checkPendingFlush(t *testing.T, f *contextDisplayCleanupFixture, original []pendingFlushArchiveEntry) {
	t.Helper()
	fragmentMust(t, f.e.flushPendingRules())
	archive, phases := pendingFlushArchive(t, f)
	if !reflect.DeepEqual(archive, original) {
		t.Fatal("flush rewrote the source formula/name/bound archive", original, archive)
	}
	for _, pending := range phases {
		if pending {
			t.Fatal("successful flush left a pending input")
		}
	}
	if len(f.e.retiredRawRules) != 0 {
		t.Fatal("successful flush left retired raw owners")
	}
	count, err := f.e.set.Len()
	fragmentMust(t, err)
	if count != 5 {
		t.Fatal("flush dropped or duplicated a normalized clause", count)
	}
	formulas, err := f.e.RuleFormulas()
	fragmentMust(t, err)
	if len(formulas) != 5 {
		t.Fatal("flushed public rule enumeration lost a clause/query", len(formulas))
	}
	fragmentMust(t, closeASTRefs(f.m, formulas))
}

func TestPendingClosedFlushAllocationFailureRetryAndClose(t *testing.T) {
	var expected [4]uint64
	t.Run("uninterrupted", func(t *testing.T) {
		f := newPendingFlushAllocationFixture(t)
		archive, _ := pendingFlushArchive(t, f)
		checkPendingFlush(t, f, archive)
		checkPendingFlush(t, f, archive)
		expected = contextDisplayLiveCounts(f.m.Stats())
	})
	if t.Failed() {
		return
	}
	failures := 0
	var completed, prefixPublished [2]bool
	for allowance := int64(0); allowance <= 2048; allowance++ {
		t.Run(fmt.Sprintf("reservations-%d", allowance), func(t *testing.T) {
			f := newPendingFlushAllocationFixture(t)
			var committed *base.Rule
			entry := int(allowance % 2)
			archive, phases := pendingFlushArchive(t, f)
			if !reflect.DeepEqual(phases, []bool{false, true, true}) {
				t.Fatal("closed flush fixture has the wrong pending phase", phases)
			}
			f.m.Budget().FailAfter(allowance)
			// Exercise both entry points: direct normalization must also drain
			// all pending inputs while publishing each completed prefix.
			var cause error
			if entry == 0 {
				cause = f.e.flushPendingRules()
			} else {
				cause = f.e.normalizePendingRules(true)
			}
			f.m.Budget().FailAfter(-1)
			if cause != nil {
				failures++
				_, interrupted := pendingFlushArchive(t, f)
				if reflect.DeepEqual(interrupted, []bool{false, false, true}) {
					prefixPublished[entry] = true
					var getErr error
					committed, getErr = f.e.set.Rule(2)
					fragmentMust(t, getErr)
				}
				var native memory.Failure
				if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &native) || native.Kind != memory.FailureMemout) {
					t.Fatal("closed flush lost the allocation cause", allowance, cause)
				}
				var receipt memory.ConstructionFailure
				if errors.As(cause, &receipt) {
					fragmentMust(t, receipt.Cleanup())
					fragmentMust(t, receipt.Cleanup())
				}
			} else {
				completed[entry] = true
			}
			current, _ := pendingFlushArchive(t, f)
			if !reflect.DeepEqual(current, archive) {
				t.Fatal("failed flush changed its original input archive", allowance, archive, current)
			}
			checkPendingFlush(t, f, archive)
			if committed != nil {
				retried, err := f.e.set.Rule(2)
				fragmentMust(t, err)
				same, err := committed.Same(retried)
				fragmentMust(t, err)
				fragmentMust(t, retried.Close())
				fragmentMust(t, committed.Close())
				if !same {
					t.Fatal("retry reconstructed the already published source prefix")
				}
			}
			checkPendingFlush(t, f, archive)
			if actual := contextDisplayLiveCounts(f.m.Stats()); actual != expected {
				t.Fatal("closed flush retry leaked AST ownership", allowance, expected, actual, cause)
			}
			fragmentMust(t, f.e.Close())
			fragmentMust(t, f.e.Close())
		})
		if t.Failed() || completed[0] && completed[1] && prefixPublished[0] && prefixPublished[1] {
			break
		}
	}
	if failures == 0 || !completed[0] || !completed[1] || !prefixPublished[0] || !prefixPublished[1] {
		t.Fatal("closed flush allocation campaign missed failure, completed prefix, or completion", failures, prefixPublished, completed)
	}
}

func TestPendingClosedFlushPushPopRestoresPhase(t *testing.T) {
	f := newPendingFlushAllocationFixture(t)
	archive, phases := pendingFlushArchive(t, f)
	before := contextDisplayLiveCounts(f.m.Stats())
	if f.e.engineInstantiated || f.e.queried {
		t.Fatal("flush scope fixture instantiated a backend")
	}
	fragmentMust(t, f.e.Push())
	checkPendingFlush(t, f, archive)
	fragmentMust(t, f.e.Pop())
	restored, restoredPhases := pendingFlushArchive(t, f)
	if !reflect.DeepEqual(restored, archive) || !reflect.DeepEqual(restoredPhases, phases) {
		t.Fatal("pop failed to restore raw input identity and pending phase", restored, restoredPhases, archive, phases)
	}
	if len(f.e.scopes) != 0 || len(f.e.rawRuleScopes) != 0 || len(f.e.assertionScopes) != 0 || f.e.popPhase != contextPopNone {
		t.Fatal("pop left a source phase or snapshot owner")
	}
	if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
		t.Fatal("flush/pop changed AST ownership", before, after)
	}
	checkPendingFlush(t, f, archive)
}

func TestPendingClosedFlushNativeRefusalPreservesPhase(t *testing.T) {
	f := newPendingFlushAllocationFixture(t)
	archive, phases := pendingFlushArchive(t, f)
	persistent := f.e.set
	before := contextDisplayLiveCounts(f.m.Stats())
	var cause error
	f.refuse(func() { cause = f.e.flushPendingRules() })
	if !errors.Is(cause, ast.ErrASTPending) {
		t.Fatal("closed flush did not refuse an active native AST invocation", cause)
	}
	current, currentPhases := pendingFlushArchive(t, f)
	if f.e.set != persistent || !reflect.DeepEqual(current, archive) || !reflect.DeepEqual(currentPhases, phases) {
		t.Fatal("native refusal partially advanced the pending flush phase")
	}
	if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
		t.Fatal("native flush refusal leaked AST owners", before, after)
	}
	checkPendingFlush(t, f, archive)
}
