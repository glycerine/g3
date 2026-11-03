package fp

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func rejectedUpdateFormula(t *testing.T, f *contextDisplayCleanupFixture) ast.ASTHandle {
	t.Helper()
	boolean := f.own(f.m.MkBoolSort())
	predicate := func(name string) ast.ASTHandle {
		h := f.own(f.m.MkFreshConst(name, boolean))
		d, err := f.m.GetDecl(h)
		fragmentMust(t, err)
		fragmentMust(t, f.e.RegisterPredicate(d))
		return h
	}
	p, q, r := predicate("reject-P"), predicate("reject-Q"), predicate("reject-R")
	fragmentMust(t, f.e.AddRuleWithBound(p, "pending-seed", 7))
	body := f.own(f.m.MkOr(p, q))
	return f.own(f.m.MkImplies(body, r))
}

func TestFixedpointRejectedUpdatePreservesRawArchiveAndPendingRanges(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	formula := rejectedUpdateFormula(t, f)
	persistent, revision := f.e.set, f.e.revision
	raw := append([]rawRuleFormula(nil), f.e.rawRules...)
	initialCount, err := persistent.Len()
	fragmentMust(t, err)
	for retry := 1; retry <= 2; retry++ {
		cause := f.e.UpdateRule(formula, "split")
		if cause == nil || !strings.Contains(cause.Error(), "non-trivial body") {
			t.Fatal("split update did not report the source rejection", cause)
		}
		count, err := persistent.Len()
		fragmentMust(t, err)
		if f.e.set != persistent || f.e.revision != revision || count != initialCount+2*retry || len(f.e.rawRules) != len(raw) {
			t.Fatal("rejected update changed context identity or lost completed clauses", count, initialCount, retry)
		}
		for i, original := range raw {
			want := original
			if want.pending {
				want.first += 2 * retry
				want.end += 2 * retry
			}
			if f.e.rawRules[i] != want {
				t.Fatal("rejected update changed raw input metadata or misplaced its eager clauses", i, f.e.rawRules[i], want)
			}
		}
	}
	// Both failed updates publish their clauses before the source duplicate
	// label check, so they must not stop at the two clauses from the first.
	fragmentMust(t, f.e.normalizePendingRules(true))
	count, err := persistent.Len()
	fragmentMust(t, err)
	if count != initialCount+4 || len(f.e.rawRules) != len(raw) {
		t.Fatal("flushing pending inputs discarded update-only clauses", count)
	}
	for i, original := range raw {
		got := f.e.rawRules[i]
		if got.formula != original.formula || got.name != original.name || got.bound != original.bound || got.pending {
			t.Fatal("flush changed original raw archive", i, got, original)
		}
	}
}

func TestFixedpointRejectedUpdatePublicationNativeCleanupRetry(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	formula := rejectedUpdateFormula(t, f)
	name, err := f.m.Symbols.Intern("split-native")
	fragmentMust(t, err)
	// Normalize this split once to stabilize the rule manager's reusable
	// HNF scratch roots before measuring temporary owner recovery.
	warm, err := base.NewRuleSet(f.e.rules)
	fragmentMust(t, err)
	fragmentMust(t, f.e.addFormula(warm, formula, name))
	fragmentMust(t, warm.Close())
	before := contextDisplayLiveCounts(f.m.Stats())
	bound, err := f.m.NewRef(formula)
	fragmentMust(t, err)
	clauses, err := base.NewRuleSet(f.e.rules)
	fragmentMust(t, err)
	outer := &contextFormulaAttempt{manager: f.m, sets: []*base.RuleSet{clauses}, references: []ast.ASTRef{bound}}
	t.Cleanup(func() { fragmentMust(t, outer.close()) })
	fragmentMust(t, f.e.addFormula(clauses, formula, name))
	count, err := clauses.Len()
	fragmentMust(t, err)
	if count != 2 {
		t.Fatal("native fixture did not normalize multiple clauses", count)
	}
	persistent, revision := f.e.set, f.e.revision
	beforeCount, err := persistent.Len()
	fragmentMust(t, err)
	raw := append([]rawRuleFormula(nil), f.e.rawRules...)
	var receipt memory.ConstructionFailure
	var inner *contextFormulaFailure
	rejection := errors.New("non-trivial update body")
	f.refuse(func() {
		// The live helper has obtained an owning rule view when the active
		// native guard refuses its first Add. Its temporary set's Close is
		// also refused, so the returned owner must survive until retry.
		cause := f.e.publishRejectedUpdate(clauses)
		if !errors.As(cause, &inner) {
			t.Fatal("publication failed without its concrete replacement owner", cause)
		}
		if got := f.e.preserveFailedUpdatePrefix(clauses, cause); got != cause {
			t.Fatal("partial-update failure path replaced an unfinished native receipt", got, cause)
		}
		if got := f.e.preserveFailedUpdatePrefix(clauses, ast.ErrASTPending); got != ast.ErrASTPending {
			t.Fatal("partial-update failure path crossed a bare native refusal", got)
		}
		// This is the caller's finalizer frontier: the inner replacement
		// must drain before the candidate set and bound formula.
		cause = outer.finish(errors.Join(rejection, cause))
		if !errors.Is(cause, rejection) || !errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &receipt) {
			t.Fatal("publication lost its native failure or temporary owner", cause)
		}
		if outer.nested != inner || len(outer.sets) != 1 || len(outer.references) != 1 {
			t.Fatal("caller cleanup crossed the refused nested owner")
		}
		if receipt.RuntimeOwner() != f.runtime || receipt.CleanupCause() == nil {
			t.Fatal("publication lost its concrete cleanup frontier")
		}
		for retry := 0; retry < 2; retry++ {
			if err := receipt.Cleanup(); !errors.Is(err, ast.ErrASTPending) {
				t.Fatal("publication cleanup crossed the active native guard", err)
			}
		}
	})
	if receipt == nil {
		t.Fatal("native guard did not exercise publication cleanup")
	}
	original, exposed, message := receipt.OriginalCause(), errors.Unwrap(receipt), receipt.Error()
	fragmentMust(t, receipt.Cleanup())
	fragmentMust(t, receipt.Cleanup())
	if receipt.CleanupCause() != nil || receipt.OriginalCause() != original || errors.Unwrap(receipt) != exposed || receipt.Error() != message {
		t.Fatal("publication cleanup changed its stable failure receipt")
	}
	if after := contextDisplayLiveCounts(f.m.Stats()); after != before {
		t.Fatal("failed publication leaked AST owners", before, after)
	}
	count, err = persistent.Len()
	fragmentMust(t, err)
	if f.e.set != persistent || count != beforeCount || f.e.revision != revision || len(raw) != len(f.e.rawRules) {
		t.Fatal("failed publication changed the persistent program")
	}
	for i := range raw {
		if raw[i] != f.e.rawRules[i] {
			t.Fatal("failed publication shifted pending rules", i)
		}
	}
	if len(outer.sets) != 0 || len(outer.references) != 0 || outer.nested != nil || len(inner.attempt.sets) != 0 {
		t.Fatal("completed cleanup left temporary ownership frontiers")
	}
	if cause := f.e.UpdateRuleWithSymbol(formula, name); cause == nil || !strings.Contains(cause.Error(), "non-trivial body") {
		t.Fatal("public update retry did not preserve its source rejection", cause)
	}
	count, err = persistent.Len()
	fragmentMust(t, err)
	if f.e.set != persistent || count != beforeCount+2 {
		t.Fatal("publication retry lost completed clauses", count)
	}
}

// A real numeral constructor receipt can own physical allocations without an
// active AST invocation. It must bypass publication independently of ErrASTPending.
func TestFixedpointFailedUpdatePreservesConstructionFailureWithoutPendingAST(t *testing.T) {
	f := newContextDisplayCleanupFixture(t)
	formula := rejectedUpdateFormula(t, f)
	name, err := f.m.Symbols.Intern("split-construction-failure")
	fragmentMust(t, err)
	clauses, err := base.NewRuleSet(f.e.rules)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, clauses.Close()) })
	fragmentMust(t, f.e.addFormula(clauses, formula, name))
	before, err := f.e.set.Len()
	fragmentMust(t, err)

	budget := arena.NewBudget(1 << 20)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
	fragmentMust(t, err)
	integers, cause := numeral.NewIntegerManager(numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget, PageSize: 2, SlotLimit: 7},
		Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{Budget: budget, PageSize: 2}},
	})
	var receipt memory.ConstructionFailure
	if integers != nil || !errors.Is(cause, arena.ErrSlotLimit) || errors.Is(cause, ast.ErrASTPending) || !errors.As(cause, &receipt) {
		t.Fatal("fixture missed an owning non-AST constructor failure", integers, cause)
	}
	t.Cleanup(func() {
		fragmentMust(t, receipt.Cleanup())
		fragmentMust(t, runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("constructor failure receipt leaked bytes", budget.Used())
		}
	})
	blocks, used := runtime.Stats().LiveBlocks, budget.Used()
	if blocks == 0 {
		t.Fatal("constructor failure has no pending physical ownership")
	}
	if got := f.e.preserveFailedUpdatePrefix(clauses, cause); got != cause {
		t.Fatal("partial-update failure replaced its native constructor receipt", got, cause)
	}
	after, err := f.e.set.Len()
	fragmentMust(t, err)
	if after != before || runtime.Stats().LiveBlocks != blocks || budget.Used() != used {
		t.Fatal("partial-update failure published clauses or consumed its native receipt", before, after)
	}
	fragmentMust(t, receipt.Cleanup())
	fragmentMust(t, receipt.Cleanup())
}
