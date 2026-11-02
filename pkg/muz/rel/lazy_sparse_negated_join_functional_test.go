package rel

import "testing"

func TestLazySparseNegatedJoinFunctionalColumnsUseReorderedJoin(t *testing.T) {
	engine := NewLazySparseEngine()
	defer engine.Close()
	target, err := engine.New(lazySignature(t, 2), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = target.Add(Fact{0})
	_, _ = target.Add(Fact{1})
	first, err := engine.New(lazySignature(t, 2, 2), 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.New(lazySignature(t, 2, 2), 1)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = first.Add(Fact{0, 0})
	_, _ = second.Add(Fact{1, 1})
	negative, err := first.Join(second, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = target.FilterByNegation(negative, []int{0}, []int{1}); err != nil {
		t.Fatal(err)
	}
	facts, err := target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0})
	if !negative.Forced() {
		t.Fatal("functional join incorrectly used raw-column negated-join fusion")
	}
}
