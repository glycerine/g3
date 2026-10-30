package rel

import (
	"errors"
	"testing"
)

func lazySignature(t *testing.T, domains ...uint64) Signature {
	t.Helper()
	signature, err := NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	return signature
}

func newLazyTableWithFacts(t *testing.T, engine *LazyTableEngine, signature Signature, facts ...Fact) *LazyTable {
	t.Helper()
	table, err := engine.New(signature)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	return table
}

func closeLazyTables(t *testing.T, tables ...*LazyTable) {
	t.Helper()
	t.Cleanup(func() {
		for _, table := range tables {
			if table != nil {
				if err := table.Close(); err != nil {
					t.Error(err)
				}
			}
		}
	})
}

func requireLazyFact(t *testing.T, table *LazyTable, fact Fact, want bool) {
	t.Helper()
	got, err := table.Contains(fact)
	if err != nil || got != want {
		t.Fatalf("fact %v: got %v want %v: %v", fact, got, want, err)
	}
}

func TestLazyTableSourceDeferredJoinAndMemoization(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(engine.Close)
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3), Fact{0, 1}, Fact{1, 2})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 3, 2), Fact{1, 1}, Fact{2, 0})
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, left, right, joined)
	if joined.Forced() || joined.Kind() != LazyTableJoin {
		t.Fatal("join was not deferred", joined.Forced(), joined.Kind())
	}
	requireLazyFact(t, joined, Fact{0, 1, 1, 1}, true)
	requireLazyFact(t, joined, Fact{1, 2, 2, 0}, true)
	first := engine.Statistics()
	if !joined.Forced() || first.Forces != 1 || first.Joins != 1 {
		t.Fatal("first force", joined.Forced(), first)
	}
	requireLazyFact(t, joined, Fact{0, 1, 1, 1}, true)
	if second := engine.Statistics(); second != first {
		t.Fatal("memoized evaluation changed statistics", first, second)
	}
}

func TestLazyTableCloneResetComplementAndLifecycle(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(engine.Close)
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 3), Fact{0}, Fact{2})
	clone, err := table.Clone()
	if err != nil {
		t.Fatal(err)
	}
	complement, err := table.Complement()
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, table, clone, complement)
	requireLazyFact(t, complement, Fact{1}, true)
	requireLazyFact(t, complement, Fact{0}, false)
	if err = table.Reset(); err != nil {
		t.Fatal("reset", err)
	}
	empty, emptyErr := table.Empty()
	if emptyErr != nil || !empty {
		t.Fatal("reset empty", empty, emptyErr)
	}
	requireLazyFact(t, clone, Fact{2}, true)
	if err = table.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = table.Facts(); !errors.Is(err, ErrLazyTable) {
		t.Fatal("closed table", err)
	}
}

func TestLazyTableMutationForcesOnlyOnObservation(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(engine.Close)
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 2), Fact{0, 0}, Fact{0, 1}, Fact{1, 1})
	engine.ResetStatistics()
	if err := table.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if table.Forced() || table.Kind() != LazyTableFilterIdentical || engine.Statistics() != (LazyTableStatistics{}) {
		t.Fatal("identical filter was eager", table.Forced(), table.Kind(), engine.Statistics())
	}
	if err := table.FilterEqual(1, 0); err != nil {
		t.Fatal(err)
	}
	if table.Forced() || table.Kind() != LazyTableFilterEqual {
		t.Fatal("equal filter was eager")
	}
	closeLazyTables(t, table)
	requireLazyFact(t, table, Fact{1, 1}, true)
	requireLazyFact(t, table, Fact{0, 0}, false)
	stats := engine.Statistics()
	if stats.Forces != 2 || stats.IdenticalFilters != 1 || stats.EqualFilters != 1 {
		t.Fatal("filter evaluation order", stats)
	}
}

func TestLazyTableIdenticalFilterAllowsDifferentDomains(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(engine.Close)
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3), Fact{0, 0}, Fact{1, 1}, Fact{1, 2})
	closeLazyTables(t, table)
	if err := table.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal("identical filter rejected columns with different domains", err)
	}
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0, 0}, Fact{1, 1})
}

func TestLazyTableJoinAllowsDifferentMatchedDomains(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(engine.Close)
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{0}, Fact{1})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 3), Fact{1}, Fact{2})
	joined, err := left.Join(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, left, right, joined)
	facts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{1, 1})
}

func TestLazyTableValidationAndEngineOwnership(t *testing.T) {
	firstEngine := NewLazyTableEngine()
	secondEngine := NewLazyTableEngine()
	t.Cleanup(firstEngine.Close)
	t.Cleanup(secondEngine.Close)
	first := newLazyTableWithFacts(t, firstEngine, lazySignature(t, 2), Fact{0})
	second := newLazyTableWithFacts(t, secondEngine, lazySignature(t, 2), Fact{0})
	closeLazyTables(t, first, second)
	if _, err := first.Join(second, nil, nil); !errors.Is(err, ErrLazyTable) {
		t.Fatal("cross-engine join", err)
	}
	if err := first.FilterEqual(2, 0); err != nil {
		t.Fatal("out-of-domain filter rejected", err)
	}
	filtered, err := first.Eval()
	if err != nil {
		t.Fatal(err)
	}
	facts, err := filtered.Facts()
	if err != nil || len(facts) != 0 {
		t.Fatal("out-of-domain filter did not remove every row", facts, err)
	}
	if err := first.FilterEqual(0, 1); !errors.Is(err, ErrLazyTable) {
		t.Fatal("out-of-range filter column", err)
	}
	if _, err := first.Project([]int{1}); err == nil {
		t.Fatal("out-of-range project accepted")
	}
	firstEngine.Close()
	if _, err := first.Eval(); !errors.Is(err, ErrLazyTable) {
		t.Fatal("closed engine", err)
	}
}
