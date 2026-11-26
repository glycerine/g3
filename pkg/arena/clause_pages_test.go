package arena

import (
	"encoding/binary"
	"testing"
)

func checkClauseBytesBudget(t *testing.T, a *ClauseBytes, b *Budget) {
	t.Helper()
	s := a.Stats()
	if total := s.OwnerBytes + s.MetadataBytes + s.PayloadBytes + s.DirectoryBytes; total != b.Used() {
		t.Fatalf("clause storage reports %d charged bytes, budget holds %d", total, b.Used())
	}
}

func TestClauseBytesOwnerBudget(t *testing.T) {
	for _, mode := range []struct {
		name   string
		config SmallBytesConfig
		pooled bool
	}{
		{"physical", SmallBytesConfig{}, true},
		{"custom", SmallBytesConfig{Alignment: 4}, false},
		{"direct", SmallBytesConfig{DirectOnly: true}, false},
	} {
		t.Run(mode.name, func(t *testing.T) {
			budget := NewBudget(1 << 20)
			cfg := mode.config
			cfg.Descriptors.Budget = budget
			for failure := int64(0); failure < 8; failure++ {
				budget.FailAfter(failure)
				a, err := NewClauseBytes(cfg)
				if err != nil {
					if err != ErrInjectedFailure || a != nil || budget.Used() != 0 {
						t.Fatalf("failed constructor at %d returned %v and left %d bytes", failure, err, budget.Used())
					}
					continue
				}
				budget.FailAfter(-1)
				checkClauseBytesBudget(t, a, budget)
				if a.RequiresDirect(244) || a.RequiresDirect(248) != mode.pooled {
					t.Fatal("incorrect direct allocation boundary")
				}
				ref, err := a.Allocate(40)
				if err != nil || !a.IsLive(ref) {
					t.Fatal("allocation", ref, err)
				}
				checkClauseBytesBudget(t, a, budget)
				a.Close()
				a.Close()
				checkClauseBytesBudget(t, a, budget)
				if budget.Used() != 0 || a.IsLive(ref) {
					t.Fatal("closed arena retains storage or a live handle")
				}
				return
			}
			t.Fatal("constructor did not complete")
		})
	}
}

func TestClauseBytesReclaimsAndReopens(t *testing.T) {
	budget := NewBudget(1 << 24)
	a, err := NewClauseBytes(SmallBytesConfig{Descriptors: Config{Budget: budget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	const size = 40
	refs := make([]SmallBytesRef, 80)
	for i := range refs {
		if i == 8 {
			before := a.Stats()
			budget.FailAfter(0)
			if _, err := a.Allocate(size); err != ErrInjectedFailure || a.Stats() != before {
				t.Fatal("refused directory growth changed state", err)
			}
			checkClauseBytesBudget(t, a, budget)
			budget.FailAfter(-1)
		}
		refs[i], err = a.Allocate(size)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.WriteUint32LE(refs[i], 0, uint32(i+1)); err != nil {
			t.Fatal(err)
		}
	}
	checkClauseBytesBudget(t, a, budget)
	original := refs[8]
	if err := a.Deallocate(size, original); err != nil {
		t.Fatal(err)
	}
	refs[8], err = a.Allocate(size)
	if err != nil || refs[8].Slot != original.Slot || refs[8].Generation <= original.Generation {
		t.Fatal("free-list reuse did not advance generation", original, refs[8], err)
	}
	for _, ref := range refs[:40] {
		if err := a.Deallocate(size, ref); err != nil {
			t.Fatal(err)
		}
	}
	before := a.Stats()
	if before.Chunks != 2 || before.Live != 40 || before.FreeObjects != 8 {
		t.Fatal("releasing two pages should keep one warm", before)
	}
	checkClauseBytesBudget(t, a, budget)
	budget.FailAfter(0)
	if _, err := a.Allocate(size); err != ErrInjectedFailure || a.Stats() != before {
		t.Fatal("refused page reactivation changed state", err)
	}
	checkClauseBytesBudget(t, a, budget)
	budget.FailAfter(-1)
	for _, old := range refs[8:40] {
		ref, err := a.Allocate(size)
		if err != nil || ref.Slot != old.Slot || ref.Generation <= old.Generation {
			t.Fatal("page reactivation lost slot or generation", ref, old, err)
		}
		if _, err := a.Borrow(old); err != ErrInvalidSlot {
			t.Fatal("stale reference survived page reactivation", old, err)
		}
	}
	checkClauseBytesBudget(t, a, budget)
	for i, ref := range refs[40:] {
		if got, err := a.ReadUint32LE(ref, 0); err != nil || got != uint32(i+41) {
			t.Fatal("neighboring live page changed", i, got, err)
		}
	}
	a.Close()
	if budget.Used() != 0 {
		t.Fatal("budget remained charged after close", budget.Used())
	}
}

func TestClauseBytesRetiresExhaustedSlot(t *testing.T) {
	budget := NewBudget(1 << 20)
	a, err := NewClauseBytes(SmallBytesConfig{Descriptors: Config{Budget: budget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	const size = 40
	var first SmallBytesRef
	for i := range 8 {
		ref, err := a.Allocate(size)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = ref
		}
	}
	first.Generation = Generation(^uint32(0))
	page := &a.pages[0]
	page.maximum = uint32(first.Generation)
	binary.LittleEndian.PutUint32(page.data, uint32(first.Generation))
	if err := a.Deallocate(size, first); err != nil {
		t.Fatal(err)
	}
	next, err := a.Allocate(size)
	if err != nil || next.Slot == first.Slot || a.IsLive(first) || a.Stats().FreeObjects != 0 {
		t.Fatal("a retired generation reentered the free list", first, next, err)
	}
	checkClauseBytesBudget(t, a, budget)
}
