package rel

import "testing"

func TestLazyTableJoinProjectFusionAgainstFiniteOracle(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3), Fact{0, 1}, Fact{1, 2})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 3, 2), Fact{1, 1}, Fact{2, 0}, Fact{0, 1})
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	projected, err := joined.Project([]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, left, right, joined, projected)
	if projected.Forced() || joined.Forced() {
		t.Fatal("join-project forced before observation")
	}
	requireLazyFact(t, projected, Fact{0, 1}, true)
	requireLazyFact(t, projected, Fact{1, 0}, true)
	requireLazyFact(t, projected, Fact{1, 1}, false)
	stats := engine.Statistics()
	if stats.JoinProjects != 1 || stats.Joins != 0 || stats.Projects != 0 || joined.Forced() {
		t.Fatal("join-project fusion", stats, joined.Forced())
	}
}

func TestLazyTableJoinProjectFusionProducesOneEmptyTuple(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	left := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{1})
	right := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{1})
	joined, err := left.Join(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	projected, err := joined.Project([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, left, right, joined, projected)
	facts, err := projected.Facts()
	if err != nil || len(facts) != 1 || len(facts[0]) != 0 {
		t.Fatalf("zero-column lazy join/project = %v, %v", facts, err)
	}
	stats := engine.Statistics()
	if stats.JoinProjects != 1 || stats.Joins != 0 || stats.Projects != 0 {
		t.Fatalf("zero-column join/project statistics = %+v", stats)
	}
}

func TestLazyTableSelectEqualProjectFusion(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3), Fact{0, 1}, Fact{1, 1}, Fact{1, 2})
	engine.ResetStatistics()
	if err := table.FilterEqual(1, 1); err != nil {
		t.Fatal(err)
	}
	projected, err := table.Project([]int{1})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, table, projected)
	requireLazyFact(t, projected, Fact{0}, true)
	requireLazyFact(t, projected, Fact{1}, true)
	if projectedRows, _ := projected.Facts(); len(projectedRows) != 2 {
		t.Fatal("selected rows", projectedRows)
	}
	stats := engine.Statistics()
	if stats.SelectEqualProject != 1 || stats.EqualFilters != 0 || stats.Projects != 0 {
		t.Fatal("select-project fusion", stats)
	}
}

func TestLazyTableSelectEqualOtherProjectionUsesFallback(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 2, 2),
		Fact{0, 1, 0}, Fact{1, 1, 1}, Fact{0, 0, 1})
	if err := table.FilterEqual(1, 1); err != nil {
		t.Fatal(err)
	}
	// The source fused factory projects away the selected column only. Removing
	// column 0 therefore takes the ordinary force/filter/project path.
	projected, err := table.Project([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, table, projected)
	if projected.Forced() {
		t.Fatal("projection forced before observation")
	}
	requireLazyFact(t, projected, Fact{1, 0}, true)
	requireLazyFact(t, projected, Fact{1, 1}, true)
	requireLazyFact(t, projected, Fact{0, 1}, false)
	stats := engine.Statistics()
	if stats.SelectEqualProject != 0 || stats.EqualFilters != 1 || stats.Projects != 1 {
		t.Fatal("non-fused select-project path", stats)
	}
}

func TestLazyTableRenameNegationAndEagerUnion(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	target := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 2), Fact{0, 0}, Fact{0, 1}, Fact{1, 1})
	negated := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{1})
	closeLazyTables(t, target, negated)
	engine.ResetStatistics()
	if err := target.FilterByNegation(negated, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if target.Forced() {
		t.Fatal("negation filter was eager")
	}
	renamed, err := target.RenameCycle([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, renamed)
	requireLazyFact(t, renamed, Fact{0, 0}, true)
	requireLazyFact(t, renamed, Fact{1, 0}, false)
	stats := engine.Statistics()
	if stats.NegationFilters != 1 || stats.Renames != 1 {
		t.Fatal("negation/rename", stats)
	}

	source := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 2), Fact{1, 0})
	delta := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 2))
	closeLazyTables(t, source, delta)
	if err = renamed.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	if !renamed.Forced() || !source.Forced() || !delta.Forced() {
		t.Fatal("source union must force all operands")
	}
	requireLazyFact(t, renamed, Fact{1, 0}, true)
	requireLazyFact(t, delta, Fact{1, 0}, true)
}

func TestLazyTableMutatingFiltersTransferCachedTables(t *testing.T) {
	for _, operation := range []string{"equal", "identical", "negation"} {
		t.Run(operation, func(t *testing.T) {
			engine := NewLazyTableEngine()
			t.Cleanup(func() {
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			})
			targetBase := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 2), Fact{0, 0}, Fact{0, 1}, Fact{1, 1})
			target, err := targetBase.Project(nil)
			if err != nil {
				t.Fatal(err)
			}
			closeLazyTables(t, targetBase, target)
			before, err := target.Eval()
			if err != nil {
				t.Fatal(err)
			}

			switch operation {
			case "equal":
				err = target.FilterEqual(1, 1)
			case "identical":
				err = target.FilterIdentical([]int{0, 1})
			case "negation":
				negative := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{1})
				closeLazyTables(t, negative)
				err = target.FilterByNegation(negative, []int{1}, []int{0})
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := target.Eval()
			if err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatal("mutating filter did not take ownership of the cached source table")
			}
		})
	}
}

func TestLazyTableBulkRemoveForcesLikeSource(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 3), Fact{0}, Fact{1}, Fact{2})
	filtered, err := table.Project(nil)
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, table, filtered)
	if filtered.Forced() {
		t.Fatal("projection forced before bulk remove")
	}
	if err = filtered.RemoveFacts([]Fact{{0}, {2}}); err != nil {
		t.Fatal(err)
	}
	if !filtered.Forced() {
		t.Fatal("bulk remove did not force the lazy table")
	}
	facts, err := filtered.Facts()
	if err != nil || len(facts) != 1 || facts[0][0] != 1 {
		t.Fatalf("rows after bulk remove = %v, %v", facts, err)
	}
}

func TestLazyTableGeneralProjectionAndRenameStatistics(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2, 3, 2), Fact{1, 2, 0})
	projected, err := table.Project([]int{1})
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := projected.RenameCycle([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	closeLazyTables(t, table, projected, renamed)
	requireLazyFact(t, renamed, Fact{0, 1}, true)
	stats := engine.Statistics()
	if stats.Projects != 1 || stats.Renames != 1 {
		t.Fatal("general transform statistics", stats)
	}
}
