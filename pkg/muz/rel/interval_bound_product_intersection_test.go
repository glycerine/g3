package rel

import (
	"math/big"
	"testing"
)

func TestIntervalBoundProductFilterByIntersection(t *testing.T) {
	target := NewIntervalBoundProduct(2, false)
	target.kinds = []string{"bound_relation", "interval_relation"}
	source := NewIntervalBoundProduct(1, true)
	astTestMust(t, source.AddFact([]*big.Rat{big.NewRat(5, 1)}))
	astTestMust(t, target.FilterByIntersection(source, []uint32{0}, []uint32{0}))
	if !target.ContainsFact([]*big.Rat{big.NewRat(5, 1), big.NewRat(100, 1)}) || target.ContainsFact([]*big.Rat{big.NewRat(4, 1), big.NewRat(100, 1)}) {
		t.Fatal("intersection join/project did not filter selected target column")
	}
	if got := target.Kinds(); len(got) != 2 || got[0] != "bound_relation" || got[1] != "interval_relation" {
		t.Fatal("intersection filter changed target component order", got)
	}
	if !source.ContainsFact([]*big.Rat{big.NewRat(5, 1)}) || source.ContainsFact([]*big.Rat{big.NewRat(4, 1)}) {
		t.Fatal("intersection filter mutated its borrowed source")
	}
}

func TestIntervalBoundProductIntersectionEmptyProjectQuirkAliasAndFailure(t *testing.T) {
	target := NewIntervalBoundProduct(1, false)
	empty := NewIntervalBoundProduct(1, true)
	astTestMust(t, target.FilterByIntersection(empty, nil, nil))
	if target.Empty() || !target.ContainsFact([]*big.Rat{big.NewRat(99, 1)}) {
		t.Fatal("C++ join/project intersection turns the empty join into a full product")
	}
	alias := NewIntervalBoundProduct(1, true)
	astTestMust(t, alias.AddFact([]*big.Rat{big.NewRat(7, 1)}))
	astTestMust(t, alias.FilterByIntersection(alias, []uint32{0}, []uint32{0}))
	if !alias.ContainsFact([]*big.Rat{big.NewRat(7, 1)}) || alias.ContainsFact([]*big.Rat{big.NewRat(8, 1)}) {
		t.Fatal("aliased intersection changed source semantics")
	}
	stable := NewIntervalBoundProduct(1, false)
	if err := stable.FilterByIntersection(NewIntervalBoundProduct(1, false), []uint32{1}, []uint32{0}); err == nil {
		t.Fatal("out-of-range intersection mapping was accepted")
	}
	if err := stable.FilterByIntersection(NewIntervalBoundProduct(1, false), []uint32{0}, nil); err == nil {
		t.Fatal("mismatched intersection mapping was accepted")
	}
	if stable.Empty() || !stable.ContainsFact([]*big.Rat{big.NewRat(-100, 1)}) {
		t.Fatal("rejected intersection partially mutated target")
	}
}
