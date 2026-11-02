package rel

import "testing"

func TestLazySparseRenameDefersAndCloneForcesIndependentBase(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	table, _ := engine.New(lazySignature(t, 2, 3), 0)
	_, _ = table.Add(Fact{1, 2})
	renamed, err := table.RenameCycle([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Forced() {
		t.Fatal("rename was eager")
	}
	clone, err := renamed.CloneFinite()
	if err != nil {
		t.Fatal(err)
	}
	if !renamed.Forced() {
		t.Fatal("source clone did not force rename")
	}
	facts, err := clone.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{2, 1})
	if err = renamed.Reset(); err != nil {
		t.Fatal(err)
	}
	if cloneFacts, cloneErr := clone.Facts(); cloneErr != nil || len(cloneFacts) != 1 {
		t.Fatal("clone shared reset owner", cloneFacts, cloneErr)
	}
}

func TestLazySparseUnionForcesSourceTargetDeltaAndNullary(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	makeDeferred := func(fact Fact) *LazySparseTable {
		base, _ := engine.New(lazySignature(t, 3), 0)
		if fact != nil {
			_, _ = base.Add(fact)
		}
		projected, err := base.Project(nil)
		if err != nil {
			t.Fatal(err)
		}
		return projected
	}
	target, source, delta := makeDeferred(Fact{0}), makeDeferred(Fact{1}), makeDeferred(nil)
	if err := target.UnionFrom(source, delta); err != nil {
		t.Fatal(err)
	}
	for _, table := range []*LazySparseTable{target, source, delta} {
		if !table.Forced() {
			t.Fatal("union did not force every operand")
		}
	}
	targetFacts, _ := target.Facts()
	deltaFacts, _ := delta.Facts()
	instructionWantFacts(t, targetFacts, Fact{0}, Fact{1})
	instructionWantFacts(t, deltaFacts, Fact{1})

	truthSeed, _ := engine.New(lazySignature(t, 2), 0)
	_, _ = truthSeed.Add(Fact{1})
	truth, _ := truthSeed.Project([]int{0})
	falseSeed, _ := engine.New(lazySignature(t, 2), 0)
	falsity, _ := falseSeed.Project([]int{0})
	if err := falsity.UnionFrom(truth, nil); err != nil {
		t.Fatal(err)
	}
	if facts, err := falsity.Facts(); err != nil || len(facts) != 1 || len(facts[0]) != 0 {
		t.Fatal("nullary union", facts, err)
	}
}
