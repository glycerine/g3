package rel

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/bitset"
	"testing"
)

func TestTBVLiveLedgerBoundedAndInvalidated(t *testing.T) {
	for _, tc := range []struct {
		name  string
		width uint32
	}{{"zero", 0}, {"nonzero", 5}} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newTestTBVManager(t, tc.width)
			a, err := m.AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			b, err := m.AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			c, err := m.AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			if err := m.Deallocate(b); err != nil {
				t.Fatal(err)
			}
			if err := m.Deallocate(c); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 10000; i++ {
				v, err := m.AllocateX()
				if err != nil {
					t.Fatal(err)
				}
				if err = m.Deallocate(v); err != nil {
					t.Fatal(err)
				}
			}
			if len(m.allocated) != 1 || cap(m.allocated) > 8 || m.allocated[0] != a {
				t.Fatalf("ledger grows with cumulative allocations: len=%d cap=%d", len(m.allocated), cap(m.allocated))
			}
			if _, err := m.String(b); !errors.Is(err, ErrTBV) {
				t.Fatalf("deallocated vector: %v", err)
			}
			m.Reset()
			if _, err := m.String(a); !errors.Is(err, ErrTBV) {
				t.Fatalf("reset vector: %v", err)
			}
			v, err := m.AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			m.Close()
			if _, err := m.String(v); !errors.Is(err, arena.ErrClosed) {
				t.Fatalf("closed manager: %v", err)
			}
		})
	}
}

func TestTBVDeallocateRefusalPreservesLedgerForRetry(t *testing.T) {
	m, _ := newTestTBVManager(t, 5)
	other, _ := newTestTBVManager(t, 5)
	v, err := m.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Deallocate(v); !errors.Is(err, ErrTBV) {
		t.Fatalf("foreign owner: %v", err)
	}
	copy := *v
	if err := m.Deallocate(&copy); !errors.Is(err, ErrTBV) {
		t.Fatalf("copied wrapper: %v", err)
	}
	ref := v.ref
	v.ref = bitset.FixedRef{} // Simulate backing-store refusal while preserving the real allocation.
	if err := m.Deallocate(v); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("backing refusal: %v", err)
	}
	if !v.live || len(m.allocated) != 1 || m.allocated[0] != v {
		t.Fatal("refusal lost vector ownership")
	}
	v.ref = ref
	if err := m.Deallocate(v); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(m.allocated) != 0 {
		t.Fatal("successful retry retained wrapper")
	}
}
