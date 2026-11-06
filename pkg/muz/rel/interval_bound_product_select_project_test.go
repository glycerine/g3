package rel

import (
	"math/big"
	"testing"
)

func TestIntervalBoundProductSelectEqualAndProject(t *testing.T) {
	p := NewIntervalBoundProduct(2, true)
	p.kinds = []string{"bound_relation", "interval_relation"}
	astTestMust(t, p.AddFact([]*big.Rat{big.NewRat(8, 1), big.NewRat(8, 1)}))
	rejected, err := p.SelectEqualAndProject(1, big.NewRat(7, 1))
	astTestMust(t, err)
	if rejected.Empty() || !rejected.ContainsFact([]*big.Rat{big.NewRat(8, 1)}) {
		t.Fatal("C++ selection empties the clone, then projection returns a full product")
	}
	selected, err := p.SelectEqualAndProject(1, big.NewRat(8, 1))
	astTestMust(t, err)
	if !selected.ContainsFact([]*big.Rat{big.NewRat(8, 1)}) || selected.ContainsFact([]*big.Rat{big.NewRat(9, 1)}) {
		t.Fatal("matching selected value did not project the remaining column")
	}
	if got := selected.Kinds(); len(got) != 2 || got[0] != "bound_relation" || got[1] != "interval_relation" {
		t.Fatal("select/project changed requested component order", got)
	}
	if !p.ContainsFact([]*big.Rat{big.NewRat(8, 1), big.NewRat(8, 1)}) || p.ContainsFact([]*big.Rat{big.NewRat(8, 1), big.NewRat(9, 1)}) {
		t.Fatal("select/project mutated its source")
	}
}

func TestIntervalBoundProductSelectProjectEmptyAndRejection(t *testing.T) {
	full := NewIntervalBoundProduct(2, false)
	if _, err := full.SelectEqualAndProject(2, big.NewRat(1, 1)); err == nil {
		t.Fatal("out-of-range selected column was accepted")
	}
	if _, err := full.SelectEqualAndProject(0, nil); err == nil {
		t.Fatal("nil selected value was accepted")
	}
	if full.Empty() || !full.ContainsFact([]*big.Rat{big.NewRat(100, 1), big.NewRat(-100, 1)}) {
		t.Fatal("rejected select/project mutated its source")
	}
	empty := NewIntervalBoundProduct(2, true)
	projected, err := empty.SelectEqualAndProject(0, big.NewRat(1, 1))
	astTestMust(t, err)
	if projected.Empty() || !projected.ContainsFact([]*big.Rat{big.NewRat(-100, 1)}) {
		t.Fatal("C++ select/project turns empty selected input into a full product")
	}
}
