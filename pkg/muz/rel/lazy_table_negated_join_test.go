package rel

import "testing"

func TestLazyTableNegatedJoinFiltersWithoutForcingSharedJoin(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	target := newLazyTableWithFacts(t, engine, lazySignature(t, 3, 2),
		Fact{0, 0}, Fact{1, 0}, Fact{2, 1})
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 3, 2),
		Fact{0, 1}, Fact{1, 0}, Fact{2, 1})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3),
		Fact{1, 0}, Fact{0, 2}, Fact{1, 2})
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, target, left, right, joined)
	if err = target.FilterByNegation(joined, []int{0, 1}, []int{0, 3}); err != nil {
		t.Fatal(err)
	}
	if target.Forced() || joined.Forced() {
		t.Fatal("negated join forced before observation")
	}
	facts, err := target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{1, 0}, Fact{2, 1})
	stats := engine.Statistics()
	if stats.NegationFilters != 1 || stats.Joins != 0 || joined.Forced() {
		t.Fatal("negated-join specialization materialized its shared join", stats, joined.Forced())
	}

	joinedFacts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	if len(joinedFacts) != 5 || !joined.Forced() {
		t.Fatal("shared join was not independently usable after filtering", joinedFacts, joined.Forced())
	}
}

func TestLazyTableNegatedJoinZeroColumnProduct(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	target := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{0}, Fact{1})
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{0})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{1})
	joined, err := left.Join(right, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, target, left, right, joined)
	if err = target.FilterByNegation(joined, nil, nil); err != nil {
		t.Fatal(err)
	}
	empty, err := target.Empty()
	if err != nil || !empty {
		t.Fatal("nonempty negated product did not clear target", empty, err)
	}
	if joined.Forced() || engine.Statistics().Joins != 0 {
		t.Fatal("zero-column specialization materialized join", engine.Statistics())
	}
}
