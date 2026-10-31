package smtcore

import (
	"math/big"
	"testing"
)

func TestIntervalRelationRangeOperations(t *testing.T) {
	q := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	a := NewRelationInterval(q(0), true, q(2), false)
	b := NewRelationInterval(q(0), false, q(2), true)
	h := a.Hull(b)
	if !h.Contains(q(0)) || !h.Contains(q(2)) {
		t.Fatal("union endpoint openness")
	}
	meet, empty := a.Meet(b)
	if empty || meet.Contains(q(0)) || meet.Contains(q(2)) || !meet.Contains(q(1)) {
		t.Fatal("intersection endpoint openness")
	}
	w := a.Widen(b)
	lo, _, hi, _ := w.Bounds()
	if lo != nil || hi != nil {
		t.Fatal("source widening must expand both endpoints")
	}
	reverse := b.Widen(a)
	lo, _, hi, _ = reverse.Bounds()
	if lo == nil || hi == nil {
		t.Fatal("source widening is asymmetric")
	}
	_, empty = a.Meet(NewRelationInterval(q(2), false, q(3), false))
	if empty {
		t.Fatal("closed endpoint is shared")
	}
	_, empty = b.Meet(NewRelationInterval(q(2), false, q(3), false))
	if !empty {
		t.Fatal("open endpoint is disjoint")
	}
	lo, _, _, _ = a.Bounds()
	lo.SetInt64(100)
	if !a.Contains(q(1)) {
		t.Fatal("returned bound aliases interval")
	}
}
