package rel

import "testing"

func TestLazyTableJoinProjectFusionAfterJoinObservation(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3), Fact{0, 1}, Fact{1, 2})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 3, 2), Fact{1, 1}, Fact{2, 0})
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, left, right, joined)
	joinedFacts, err := joined.Facts()
	if err != nil || len(joinedFacts) != 2 {
		t.Fatal("initial join observation", joinedFacts, err)
	}
	engine.ResetStatistics()
	projected, err := joined.Project([]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, projected)
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0, 1}, Fact{1, 0})
	stats := engine.Statistics()
	if stats.JoinProjects != 1 || stats.Joins != 0 || stats.Projects != 0 {
		t.Fatal("cached join bypassed source fusion", stats)
	}
	if current, err := joined.Facts(); err != nil || len(current) != 2 {
		t.Fatal("cached join owner changed during fused projection", current, err)
	}
}

func TestLazyTableSelectEqualProjectFusionAfterFilterObservation(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3),
		Fact{0, 1}, Fact{1, 1}, Fact{1, 2})
	closeLazyTables(t, table)
	if err := table.FilterEqual(1, 1); err != nil {
		t.Fatal(err)
	}
	filtered, err := table.Facts()
	if err != nil || len(filtered) != 2 {
		t.Fatal("initial filter observation", filtered, err)
	}
	engine.ResetStatistics()
	projected, err := table.Project([]int{1})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, projected)
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0}, Fact{1})
	stats := engine.Statistics()
	if stats.SelectEqualProject != 1 || stats.EqualFilters != 0 || stats.Projects != 0 {
		t.Fatal("cached equality filter bypassed source fusion", stats)
	}
	if current, err := table.Facts(); err != nil || len(current) != 2 {
		t.Fatal("cached filter owner changed during fused projection", current, err)
	}
}
