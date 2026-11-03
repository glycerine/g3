package rel

import (
	"math/big"
	"testing"
)

func TestIntervalBoundProductResetUsesComponentFilters(t *testing.T) {
	m := astTestManager(t)
	p := NewIntervalBoundProduct(2, false)
	p.kinds = []string{"bound_relation", "interval_relation"}
	astTestMust(t, p.bound.AddLess(0, 1))
	astTestMust(t, p.Reset(m))
	if !p.Empty() || !p.interval.Empty() || !p.bound.Empty() {
		t.Fatal("false interpreted filter did not reset every product component")
	}
	if got := p.Kinds(); len(got) != 2 || got[0] != "bound_relation" || got[1] != "interval_relation" {
		t.Fatal("reset changed requested component order", got)
	}
	astTestMust(t, p.AddFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(1, 1)}))
	if p.Empty() || !p.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(1, 1)}) {
		t.Fatal("reset product retained an old bound or could not accept a later fact")
	}
}

func TestIntervalBoundProductResetNullaryAndRejection(t *testing.T) {
	m := astTestManager(t)
	unit := NewIntervalBoundProduct(0, false)
	astTestMust(t, unit.Reset(m))
	if !unit.Empty() {
		t.Fatal("reset did not empty nullary product")
	}
	stable := NewIntervalBoundProduct(1, false)
	if err := stable.Reset(nil); err == nil {
		t.Fatal("nil reset manager was accepted")
	}
	if stable.Empty() || !stable.ContainsFact([]*big.Rat{big.NewRat(100, 1)}) {
		t.Fatal("rejected reset mutated product")
	}
	var absent *IntervalBoundProduct
	if err := absent.Reset(m); err == nil {
		t.Fatal("nil product reset was accepted")
	}
}

func TestIntervalBoundProductInterpretedFilterDispatchesInRequestedOrder(t *testing.T) {
	m := astTestManager(t)
	bottom, err := m.MkFalse()
	astTestMust(t, err)
	astTestPin(t, m, bottom)

	intervalFirst := NewIntervalBoundProduct(1, false)
	intervalFirst.kinds = []string{"interval_relation", "bound_relation"}
	intervalFirst.bound = nil
	if err := intervalFirst.FilterInterpreted(m, bottom); err == nil {
		t.Fatal("malformed second bound component was accepted")
	}
	if !intervalFirst.interval.Empty() {
		t.Fatal("first interval component was not filtered before second component failed")
	}

	boundFirst := NewIntervalBoundProduct(1, false)
	boundFirst.kinds = []string{"bound_relation", "interval_relation"}
	boundFirst.interval = nil
	if err := boundFirst.FilterInterpreted(m, bottom); err == nil {
		t.Fatal("malformed second interval component was accepted")
	}
	if !boundFirst.bound.Empty() {
		t.Fatal("first bound component was not filtered before second component failed")
	}

	reset := NewIntervalBoundProduct(1, false)
	reset.kinds = []string{"bound_relation", "interval_relation"}
	reset.interval = nil
	if err := reset.Reset(m); err == nil || !reset.bound.Empty() {
		t.Fatal("reset did not delegate to the ordered interpreted filter", err)
	}
}
