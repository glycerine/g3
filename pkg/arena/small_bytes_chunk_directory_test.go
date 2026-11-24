package arena

import (
	"errors"
	"testing"
)

func TestSmallBytesReusesLowestVacantChunkAcrossFailedAllocation(t *testing.T) {
	budget := NewBudget(1 << 20)
	store := newSmallTest(t, 70, budget)
	first := smallAlloc(t, store, 256)
	var pooled [4]SmallBytesRef
	for i := range pooled {
		pooled[i] = smallAlloc(t, store, 16)
	}
	survivor := smallAlloc(t, store, 256)
	last := smallAlloc(t, store, 256)
	if err := store.Write(survivor, 0, 77); err != nil {
		t.Fatal(err)
	}
	smallFree(t, store, 256, last)
	for _, r := range pooled {
		smallFree(t, store, 16, r)
	}
	if err := store.Consolidate(); err != nil {
		t.Fatal(err)
	}
	smallFree(t, store, 256, first)
	used := budget.Used()
	budget.FailAfter(0)
	if _, err := store.Allocate(256); !errors.Is(err, ErrInjectedFailure) || budget.Used() != used {
		t.Fatalf("failed allocation used %d bytes above baseline, error %v", int64(budget.Used())-int64(used), err)
	}
	budget.FailAfter(-1)
	for _, want := range []uint32{0, 1, 3, 4} {
		r := smallAlloc(t, store, 256)
		if page := smallLocation(t, store, r)[0]; page != want {
			t.Fatalf("allocated chunk %d, want %d", page, want)
		}
	}
	if value, err := store.Read(survivor, 0); err != nil || value != 77 {
		t.Fatal("surviving chunk", value, err)
	}
	smallFree(t, store, 256, survivor)
	store.Reset()
	r := smallAlloc(t, store, 256)
	if page := smallLocation(t, store, r)[0]; page != 0 || store.IsLive(first) || store.IsLive(survivor) {
		t.Fatalf("reset allocated chunk %d, old handles live %v, %v", page, store.IsLive(first), store.IsLive(survivor))
	}
}
