package heap

import (
	"math"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// TestActivityHeapMatchesLessHeap drives an Activity heap and a heap with
// the equivalent Less closure through the same operations and compares
// every observable result.
func TestActivityHeapMatchesLessHeap(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	var act []float64
	const n = 300
	for i := 0; i < n; i++ {
		act = append(act, float64(r.Intn(40)))
	}
	a, err := New(n, Config{Activity: &act})
	heapMust(t, err)
	l, err := New(n, Config{Less: func(x, y int32) bool { return act[x] > act[y] }})
	heapMust(t, err)
	defer func() { heapMust(t, a.Close()); heapMust(t, l.Close()) }()
	for step := 0; step < 20000; step++ {
		v := int32(r.Intn(n))
		switch r.Intn(7) {
		case 0, 1:
			if !a.Contains(v) {
				heapMust(t, a.Insert(v))
				heapMust(t, l.Insert(v))
			}
		case 2:
			if !a.Empty() {
				x, e1 := a.EraseMin()
				y, e2 := l.EraseMin()
				heapMust(t, e1)
				heapMust(t, e2)
				if x != y {
					t.Fatal("EraseMin", step, x, y)
				}
			}
		case 3:
			if a.Contains(v) {
				act[v] += float64(r.Intn(5))
				heapMust(t, a.Decreased(v))
				heapMust(t, l.Decreased(v))
			}
		case 4:
			if a.Contains(v) {
				heapMust(t, a.Erase(v))
				heapMust(t, l.Erase(v))
			}
		case 5:
			heapMust(t, a.EraseIfPresent(v))
			if l.Contains(v) {
				heapMust(t, l.Erase(v))
			}
		case 6:
			if a.Contains(v) {
				act[v] -= float64(r.Intn(5))
				heapMust(t, a.Increased(v))
				heapMust(t, l.Increased(v))
			}
		}
		if a.Size() != l.Size() || !a.CheckInvariant() {
			t.Fatal("size/invariant", step)
		}
		for i := range a.values {
			if a.values[i] != l.values[i] {
				t.Fatal("values differ", step, i)
			}
		}
	}
}

func TestEraseIfPresentKeepsAbsentHeapAndGuardsPosition(t *testing.T) {
	for _, activity := range []bool{false, true} {
		scores := []float64{math.NaN(), 0, 1, 1}
		config := Config{}
		if activity {
			config.Activity = &scores
		}
		h, e := New(4, config)
		heapMust(t, e)
		heapMust(t, h.Insert(1))
		heapMust(t, h.Insert(2))
		epoch := h.epoch
		for _, absent := range []int32{-1, 0, 4} {
			heapMust(t, h.EraseIfPresent(absent))
		}
		if h.epoch != epoch {
			t.Fatal("absent value advanced epoch")
		}
		saved := h.indices[1]
		for _, invalid := range []int32{-1, 99} {
			h.indices[1] = invalid
			// Debug assertions on a deliberately corrupted heap run before
			// the bounds guard, so compare with the checked Erase entry point.
			want := h.Erase(1)
			if got := h.EraseIfPresent(1); got != want || got == nil || h.epoch != epoch {
				t.Fatal("invalid position", invalid, got, want)
			}
		}
		h.indices[1] = saved
		h.epoch = math.MaxUint64
		heapMust(t, h.EraseIfPresent(0))
		if got := h.EraseIfPresent(1); got != arena.ErrSlotLimit || !h.Contains(1) {
			t.Fatal("epoch exhaustion", got)
		}
		h.epoch = epoch
		copied := *h
		heapMust(t, copied.EraseIfPresent(1))
		heapMust(t, h.EraseIfPresent(1))
		if h.Contains(1) || h.epoch != epoch+1 {
			t.Fatal("present value survived removal")
		}
		heapMust(t, h.Close())
		heapMust(t, h.EraseIfPresent(2))
	}
	var absent *Int
	heapMust(t, absent.EraseIfPresent(1))
}

func TestActivityEraseWithUnorderedScores(t *testing.T) {
	scores := []float64{1, math.NaN(), 1, 1, math.NaN(), 2, -1}
	a, e := New(int32(len(scores)), Config{Activity: &scores})
	heapMust(t, e)
	b, e := New(int32(len(scores)), Config{Less: func(i, j int32) bool { return scores[i] > scores[j] }})
	heapMust(t, e)
	defer func() { heapMust(t, a.Close()); heapMust(t, b.Close()) }()
	for i := range scores {
		heapMust(t, a.Insert(int32(i)))
		heapMust(t, b.Insert(int32(i)))
	}
	for _, v := range []int32{2, 5, 0, 0, 3, 1, 6, 4} {
		heapMust(t, a.EraseIfPresent(v))
		if b.Contains(v) {
			heapMust(t, b.Erase(v))
		}
		for i, x := range b.values {
			if a.values[i] != x {
				t.Fatal("replacement order", v, a.values, b.values)
			}
		}
	}
}
