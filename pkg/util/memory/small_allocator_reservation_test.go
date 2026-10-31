package memory

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSmallAllocatorReservationCountsReusableLocations(t *testing.T) {
	a, _, budget := smallObjectFixture(t, SmallAllocatorProfile{PointerBits: 64})
	smallObjectMust(t, a.reserveLocations(4))
	a.locations = a.locations[:cap(a.locations)]
	for i := range a.locations {
		a.locations[i].live = true
	}
	a.locations[0].live = false
	a.locations[1].live, a.locations[1].retired = false, true
	t.Cleanup(func() { a.locations = a.locations[:0] })
	capacity, used := cap(a.locations), budget.Used()
	budget.FailAfter(0)
	if err := a.reserveLocations(1); err != nil {
		t.Fatal("reusable slot should require no reservation", err)
	}
	if err := a.reserveLocations(2); !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("retired slot must not count as reusable", err)
	}
	if cap(a.locations) != capacity || budget.Used() != used {
		t.Fatal("failed growth changed the location ledger")
	}
	budget.FailAfter(-1)
	smallObjectMust(t, a.reserveLocations(2))
	if cap(a.locations) <= capacity {
		t.Fatal("insufficient reusable locations did not grow the ledger")
	}
}

func TestSmallAllocatorReusesLowestReleasedDirectLocation(t *testing.T) {
	a, _, _ := smallObjectFixture(t, SmallAllocatorProfile{PointerBits: 64})
	first := smallObjectAlloc(t, a, 300)
	middle := smallObjectAlloc(t, a, 300)
	last := smallObjectAlloc(t, a, 300)
	smallObjectMust(t, a.Deallocate(300, middle))
	reusedMiddle := smallObjectAlloc(t, a, 300)
	if reusedMiddle.slot != middle.slot || reusedMiddle.generation == middle.generation {
		t.Fatal("middle slot was not safely reused", middle, reusedMiddle)
	}
	if _, err := a.object(middle); !errors.Is(err, ErrAllocatorOwner) {
		t.Fatal("stale reference accepted", err)
	}
	smallObjectMust(t, a.Deallocate(300, first))
	smallObjectMust(t, a.Deallocate(300, reusedMiddle))
	reusedFirst := smallObjectAlloc(t, a, 300)
	if reusedFirst.slot != first.slot || reusedFirst.generation == first.generation {
		t.Fatal("lowest released slot was not reused", first, reusedFirst)
	}
	smallObjectMust(t, a.Deallocate(300, reusedFirst))
	smallObjectMust(t, a.Deallocate(300, last))
}

func TestSmallAllocatorReusesLocationsAfterChunkReclamation(t *testing.T) {
	a, _, _ := smallObjectFixture(t, SmallAllocatorProfile{PointerBits: 64})
	refs := make([]ObjectRef, 340)
	for i := range refs {
		refs[i] = smallObjectAlloc(t, a, 17)
	}
	for _, ref := range refs {
		smallObjectMust(t, a.Deallocate(17, ref))
	}
	smallObjectMust(t, a.Consolidate())
	if a.Stats().Chunks != 0 {
		t.Fatal("source chunk was not reclaimed")
	}
	first := smallObjectAlloc(t, a, 300)
	second := smallObjectAlloc(t, a, 300)
	if first.slot != 1 || second.slot != refs[0].slot || second.generation == refs[0].generation {
		t.Fatal("reclaimed locations not reused in order", first, second, refs[0])
	}
	if _, err := a.object(refs[0]); !errors.Is(err, ErrAllocatorOwner) {
		t.Fatal("stale pooled reference accepted", err)
	}
	smallObjectMust(t, a.Deallocate(300, first))
	smallObjectMust(t, a.Deallocate(300, second))
}
