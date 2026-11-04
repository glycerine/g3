package rel

import (
	"math/big"
	"testing"
)

// Source int2ints iterates used hash-table slots, not occurrence order. For
// variable indexes 1 then 0, the initial eight-slot table visits 0 then 1.
func TestKarrIdenticalFilterSourceOrderIsObservable(t *testing.T) {
	source := NewKarrFull(4, nil)
	firstSeen := NewKarrFull(4, nil)
	for _, p := range []struct {
		r      *KarrRelation
		groups [][]uint32
	}{
		{source, [][]uint32{{2, 3}, {0, 1}}}, {firstSeen, [][]uint32{{0, 1}, {2, 3}}},
	} {
		for _, group := range p.groups {
			if err := p.r.FilterIdentical(group); err != nil {
				t.Fatal(err)
			}
		}
	}
	s, err := source.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	f, err := firstSeen.Inequalities()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.A) != 2 || len(f.A) != 2 || s.A[0][2].Cmp(big.NewRat(1, 1)) != 0 || f.A[0][0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("inequality order", s, f)
	}
	sb, err := source.Basis()
	if err != nil {
		t.Fatal(err)
	}
	fb, err := firstSeen.Basis()
	if err != nil {
		t.Fatal(err)
	}
	if len(sb.A) < 2 || len(fb.A) < 2 || sb.A[1][2].Cmp(big.NewRat(1, 1)) != 0 || fb.A[1][0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("Hilbert generator order did not follow the different filter order", sb, fb)
	}
}
