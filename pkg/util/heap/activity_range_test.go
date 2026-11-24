package heap

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestDescendingActivityErase(t *testing.T) {
	if diagnostic.DebugBuild {
		t.Skip("debug heaps perform scalar assertions on each erase")
	}
	const n = 72
	rng := rand.New(rand.NewSource(79))
	for sample := 0; sample < 350; sample++ {
		scores := make([]float64, n)
		for i := range scores {
			scores[i] = float64(rng.Intn(4))
			if i%17 == 0 {
				scores[i] = math.NaN()
			}
		}
		got, err := New(n, Config{Activity: &scores})
		heapMust(t, err)
		want, err := New(n, Config{Less: func(i, j int32) bool { return scores[i] > scores[j] }})
		heapMust(t, err)
		for _, i := range rng.Perm(n) {
			if rng.Intn(4) != 0 {
				heapMust(t, got.Insert(int32(i)))
				heapMust(t, want.Insert(int32(i)))
			}
		}
		lo, hi := rng.Intn(n), rng.Intn(n+1)
		if lo > hi {
			lo, hi = hi, lo
		}
		switch sample % 4 {
		case 0:
			lo, hi = 0, n
		case 1:
			lo, hi = 1, n
		case 2:
			lo, hi = 0, n-1
		}
		prepared, ok := got.PrepareActivityErase(&scores, lo, hi)
		if !ok {
			t.Fatal("activity range declined", sample)
		}
		// The AST suffix can stop before reaching the prepared lower bound.
		if sample%7 == 0 && hi > lo {
			lo += rng.Intn(hi - lo + 1)
		}
		prepared.EraseSuffix(hi - lo)
		for i := hi - 1; i >= lo; i-- {
			heapMust(t, want.EraseIfPresent(int32(i)))
		}
		if got.epoch != want.epoch || !slices.Equal(got.values, want.values) || !slices.Equal(got.indices, want.indices) {
			t.Fatalf("sample %d range [%d,%d), heap %v, want %v", sample, lo, hi, got.values, want.values)
		}
		heapMust(t, got.Close())
		heapMust(t, want.Close())
	}
}

func TestDescendingActivityErasePreflight(t *testing.T) {
	scores := []float64{0, 1, 0}
	h, e := New(3, Config{Activity: &scores})
	heapMust(t, e)
	defer func() { heapMust(t, h.Close()) }()
	heapMust(t, h.Insert(1))
	epoch := h.epoch
	other := slices.Clone(scores)
	for _, tc := range []struct {
		scores    *[]float64
		low, high int
	}{{&other, 0, 1}, {nil, 0, 1}, {&scores, -1, 1}, {&scores, 2, 1}, {&scores, 1, 4}} {
		if _, ok := h.PrepareActivityErase(tc.scores, tc.low, tc.high); ok {
			t.Fatal("invalid activity preflight", tc)
		}
	}
	h.epoch = math.MaxUint64 - 1
	if _, ok := h.PrepareActivityErase(&scores, 0, 2); ok {
		t.Fatal("epoch overflow")
	}
	one, ok := h.PrepareActivityErase(&scores, 1, 2)
	if ok == diagnostic.DebugBuild || !h.Contains(1) {
		t.Fatal("epoch preflight changed membership")
	}
	if ok {
		one.EraseSuffix(1)
		if h.epoch != math.MaxUint64 || h.Contains(1) {
			t.Fatal("last available epoch")
		}
	}
	h.epoch = epoch
	var none *Int
	empty, ok := none.PrepareActivityErase(&scores, 0, 1)
	if !ok {
		t.Fatal("absent optional queue")
	}
	empty.EraseSuffix(1)
}

func TestDescendingActivityEraseRejectsInvalidUse(t *testing.T) {
	if diagnostic.DebugBuild {
		t.Skip("debug heaps perform scalar assertions on each erase")
	}
	scores := []float64{0, 0, 0}
	h, err := New(3, Config{Activity: &scores})
	heapMust(t, err)
	defer func() { heapMust(t, h.Close()) }()
	heapMust(t, h.Insert(1))
	prepared, ok := h.PrepareActivityErase(&scores, 0, 2)
	if !ok {
		t.Fatal("prepare erase")
	}
	checkPanic := func(name string, count int) {
		t.Run(name, func(t *testing.T) {
			epoch := h.epoch
			defer func() {
				if recover() == nil {
					t.Error("invalid erase returned")
				}
				if h.epoch != epoch || !h.Contains(1) {
					t.Error("invalid erase changed the heap")
				}
			}()
			prepared.EraseSuffix(count)
		})
	}
	checkPanic("Count", 3)
	heapMust(t, h.Insert(2))
	checkPanic("Stale", 1)
}
