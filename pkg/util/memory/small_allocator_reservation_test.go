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
