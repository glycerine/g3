package heap

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestReservedActivityInsertionMatchesInsert(t *testing.T) {
	const n = 384
	rng := rand.New(rand.NewSource(854))
	scores := make([]float64, n)
	for i := range scores {
		scores[i] = float64(rng.Intn(11) - 5)
	}
	scores[2], scores[3] = math.NaN(), math.Inf(-1)
	makeHeap := func() *Int {
		h, err := New(n, Config{Activity: &scores})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := h.Close(); err != nil {
				t.Fatal(err)
			}
		})
		return h
	}
	a, b := makeHeap(), makeHeap()
	// Give both heaps room without changing their inverse index or priorities.
	for i := range n {
		if err := a.Insert(int32(i)); err != nil {
			t.Fatal(err)
		}
		if err := b.Insert(int32(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := b.Reset(); err != nil {
		t.Fatal(err)
	}
	for _, i := range rng.Perm(n) {
		ok := a.TryInsertReservedActivity(int32(i))
		if ok == diagnostic.DebugBuild {
			t.Fatal("unexpected fast insertion", ok)
		}
		if !ok {
			if err := a.Insert(int32(i)); err != nil {
				t.Fatal(err)
			}
		}
		if err := b.Insert(int32(i)); err != nil {
			t.Fatal(err)
		}
		if a.epoch != b.epoch || !slices.Equal(a.values, b.values) || !slices.Equal(a.indices, b.indices) {
			t.Fatal("heap or inverse positions differ after inserting", i)
		}
		if a.TryInsertReservedActivity(int32(i)) {
			t.Fatal("duplicate", i)
		}
	}
	for range n {
		x, e := a.EraseMin()
		if e != nil {
			t.Fatal(e)
		}
		y, e := b.EraseMin()
		if e != nil || x != y {
			t.Fatal("priority order", x, y, e)
		}
	}
	epoch := a.epoch
	copied := *a
	if a.TryInsertReservedActivity(-1) || a.TryInsertReservedActivity(n) || copied.TryInsertReservedActivity(1) || (*Int)(nil).TryInsertReservedActivity(0) {
		t.Fatal("accepted invalid owner or index")
	}
	a.epoch = ^uint64(0)
	if a.TryInsertReservedActivity(1) {
		t.Fatal("wrapped epoch")
	}
	a.epoch = epoch
	values := a.values
	a.values = a.values[:len(a.values):len(a.values)]
	if a.TryInsertReservedActivity(1) {
		t.Fatal("grew storage")
	}
	a.values = values
	fullScores := scores
	scores = scores[:0]
	if a.TryInsertReservedActivity(1) {
		t.Fatal("accepted missing score")
	}
	scores = fullScores
	if a.epoch != epoch || len(a.values) != 1 || a.indices[1] != 0 {
		t.Fatal("refusal changed heap")
	}
}
