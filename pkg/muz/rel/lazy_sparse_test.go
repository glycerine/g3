package rel

import "testing"

func TestLazySparseJoinProjectUsesConcreteSparseFusion(t *testing.T) {
	engine := NewLazySparseEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	left, err := engine.New(lazySignature(t, 2, 3), 0)
	if err != nil {
		t.Fatal(err)
	}
	right, err := engine.New(lazySignature(t, 3, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []Fact{{0, 1}, {1, 2}} {
		if _, err = left.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	for _, fact := range []Fact{{1, 1}, {2, 0}} {
		if _, err = right.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := left.Join(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	projected, err := joined.Project([]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if joined.Forced() || projected.Forced() {
		t.Fatal("sparse join/project was eager")
	}
	facts, err := projected.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0, 1}, Fact{1, 0})
	stats := engine.Statistics()
	if stats.JoinProjects != 1 || stats.Joins != 0 || stats.Projects != 0 || joined.Forced() {
		t.Fatal("sparse join/project did not use deferred fusion", stats, joined.Forced())
	}
	if projected.root.evaluated == nil || projected.root.evaluated.layout.entryBytes == 0 {
		t.Fatal("fused result is not concrete sparse storage")
	}
	if err = projected.Close(); err != nil {
		t.Fatal(err)
	}
	if err = joined.Close(); err != nil {
		t.Fatal(err)
	}
	if err = left.Close(); err != nil {
		t.Fatal(err)
	}
	if err = right.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLazySparseCapabilityAndFunctionalMetadata(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	if _, err := engine.New(lazySignature(t), 0); err == nil {
		t.Fatal("source sparse plugin accepted zero-arity signature")
	}
	table, err := engine.New(lazySignature(t, 2, 3), 1)
	if err != nil {
		t.Fatal(err)
	}
	functional, err := table.FunctionalColumns()
	if err != nil || functional != 1 || table.root.evaluated == nil {
		t.Fatal("functional sparse base metadata", functional, err)
	}
}
