package numeral

// Concrete numeric-cell storage adaptation tests.

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestNumericCellSlabConcreteLifecycle(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	s, e := newNumericCellSlab(arena.Config{PageSize: 2, SlotLimit: 5, Budget: budget})
	if e != nil {
		t.Fatal(e)
	}
	slots := make([]arena.Slot, 5)
	gens := make([]arena.Generation, 5)
	for i := range slots {
		slots[i], gens[i], e = s.Allocate()
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Write(slots[i], NumericCell{Size: uint32(i + 1), Capacity: 6}); e != nil {
			t.Fatal(e)
		}
	}
	if _, _, e = s.Allocate(); !errors.Is(e, arena.ErrSlotLimit) {
		t.Fatal(e)
	}
	for i, slot := range slots {
		c, e := s.Read(slot)
		if e != nil || c.Size != uint32(i+1) || !s.IsLive(slot, gens[i]) {
			t.Fatal(c, e)
		}
	}
	if e = s.Recycle(slots[1]); e != nil {
		t.Fatal(e)
	}
	if s.IsLive(slots[1], gens[1]) {
		t.Fatal("dead generation live")
	}
	slot, generation, e := s.Allocate()
	if e != nil || slot != slots[1] || generation != gens[1]+1 {
		t.Fatal(slot, generation, e)
	}
	if c, e := s.Read(slot); e != nil || c != (NumericCell{}) {
		t.Fatal("recycled cell not reset", c, e)
	}
	s.Reset()
	if s.Stats().Live != 0 || s.Stats().RecordBytes != 0 {
		t.Fatal(s.Stats())
	}
	slot, generation, e = s.Allocate()
	if e != nil || !s.IsLive(slot, generation) {
		t.Fatal(e)
	}
	s.Close()
	s.Close()
	if budget.Used() != 0 {
		t.Fatal("close retained budget", budget.Used())
	}
	if _, _, e = s.Allocate(); !errors.Is(e, arena.ErrClosed) {
		t.Fatal(e)
	}
}

func TestNumericCellSlabGenerationRetirement(t *testing.T) {
	s, e := newNumericCellSlab(arena.Config{PageSize: 1, SlotLimit: 1})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	slot, _, e := s.Allocate()
	if e != nil {
		t.Fatal(e)
	}
	s.State(slot).Generation = arena.Generation(math.MaxUint32)
	if e = s.Recycle(slot); e != nil {
		t.Fatal(e)
	}
	if s.Stats().Retired != 1 {
		t.Fatal(s.Stats())
	}
	if _, _, e = s.Allocate(); !errors.Is(e, arena.ErrSlotLimit) {
		t.Fatal(e)
	}
}

func TestNumericCellSlabFailureAndReuseAllocations(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	s, e := newNumericCellSlab(arena.Config{PageSize: 1, Budget: budget})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	budget.FailAfter(0)
	if _, _, e = s.Allocate(); !errors.Is(e, arena.ErrInjectedFailure) || budget.Used() != s.Stats().OwnerBytes || s.Stats().Live != 0 {
		t.Fatal(e, s.Stats())
	}
	budget.FailAfter(-1)
	slot, _, e := s.Allocate()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Recycle(slot); e != nil {
		t.Fatal(e)
	}
	var allocationError error
	allocations := testing.AllocsPerRun(100, func() {
		slot, _, allocationError = s.Allocate()
		if allocationError == nil {
			allocationError = s.Write(slot, NumericCell{Size: 1, Capacity: 6})
		}
		if allocationError == nil {
			allocationError = s.Recycle(slot)
		}
	})
	if allocationError != nil || allocations != 0 {
		t.Fatal("recycling allocates", allocations, allocationError)
	}
}

func TestIntegerConcreteStorageWarmArithmeticAllocations(t *testing.T) {
	m, _ := integerManager(t)
	var a, b, out Int
	if e := m.SetInt64(&a, 1<<40); e != nil {
		t.Fatal(e)
	}
	if e := m.SetInt64(&b, 1<<39); e != nil {
		t.Fatal(e)
	}
	if e := m.SetInt64(&out, 1<<40); e != nil {
		t.Fatal(e)
	}
	var operationError error
	allocations := testing.AllocsPerRun(100, func() {
		operationError = m.Add(&a, &b, &out)
		if operationError == nil {
			operationError = m.Sub(&a, &b, &out)
		}
	})
	if operationError != nil || allocations != 0 {
		t.Fatal("warm Add/Sub allocates", allocations, operationError)
	}
	if value, e := m.Int64(&out); e != nil || value != 1<<39 {
		t.Fatal(value, e)
	}
}

func TestNumericCellSlabDirectoryPeakBudget(t *testing.T) {
	// Budget permits the final two-page layout but not overlapping old and new
	// directory arrays during growth. This preserves the original peak charge.
	pageBytes := uint64(reflect.TypeOf(NumericCell{}).Size() + reflect.TypeOf(numericCellSlotState{}).Size())
	directoryEntry := uint64(reflect.TypeOf(numericCellPage{}).Size())
	budget := arena.NewBudget(uint64(reflect.TypeOf(numericCellSlab{}).Size()) + 2*pageBytes + 2*directoryEntry)
	s, e := newNumericCellSlab(arena.Config{PageSize: 1, Budget: budget})
	if e != nil {
		t.Fatal(e)
	}
	slot, generation, e := s.Allocate()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Write(slot, NumericCell{Size: 7, Capacity: 9}); e != nil {
		t.Fatal(e)
	}
	before := s.Stats()
	used := budget.Used()
	if _, _, e = s.Allocate(); !errors.Is(e, arena.ErrBudget) {
		t.Fatal("growth did not charge old+new directory", e)
	}
	if s.Stats() != before || budget.Used() != used || !s.IsLive(slot, generation) {
		t.Fatal("failed growth changed storage")
	}
	if c, e := s.Read(slot); e != nil || c.Size != 7 {
		t.Fatal(c, e)
	}
	s.Close()
	if budget.Used() != 0 {
		t.Fatal(budget.Used())
	}
}

func TestNumericCellSlabShortFirstPage(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	s, err := newNumericCellSlab(arena.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var first *numericCellSlotState
	for i := 1; i <= 1025; i++ {
		if i == 65 || i == 1025 {
			before, used := s.Stats(), budget.Used()
			budget.FailAfter(0)
			if _, _, err := s.Allocate(); !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal("failed page growth", i, err)
			}
			if s.Stats() != before || budget.Used() != used {
				t.Fatal("failed page growth changed the slab", i)
			}
			budget.FailAfter(-1)
		}
		slot, gen, err := s.Allocate()
		if err != nil || int(slot) != i || gen != 1 {
			t.Fatal(i, slot, gen, err)
		}
		if err := s.Write(slot, NumericCell{Size: uint32(i)}); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			first = s.State(slot)
			if len(s.pages[0].records) != 64 {
				t.Fatal("first page capacity", len(s.pages[0].records))
			}
		}
	}
	if first != s.State(1) || len(s.pages) != 2 || len(s.pages[0].records) != 1024 || len(s.pages[1].records) != 1024 {
		t.Fatal("unexpected page dimensions or moved metadata", s.Stats())
	}
	for i := 1; i <= 1025; i++ {
		if value, err := s.Read(arena.Slot(i)); err != nil || value.Size != uint32(i) {
			t.Fatal("lost cell after growth", i, value.Size, err)
		}
	}
	page, index := s.location(arena.Slot(math.MaxUint32))
	if page != (1<<22)-1 || index != 1022 {
		t.Fatal("wrapped high slot", page, index)
	}
	s.Reset()
	if stats := s.Stats(); stats.Live != 0 || stats.Pages != 0 || stats.RecordBytes != 0 {
		t.Fatal(stats)
	}
	s.Close()
	if budget.Used() != 0 {
		t.Fatal("close retained budget", budget.Used())
	}
}

func TestNumericCellSlabTrimAndRefillShortPage(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	s, err := newNumericCellSlab(arena.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	slot, previous, err := s.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	used := budget.Used()
	if err := s.Recycle(slot); err != nil {
		t.Fatal(err)
	}
	s.TrimFreePages()
	if used-budget.Used() != 64*s.recordBytes {
		t.Fatal("unexpected short-page charge", used, budget.Used())
	}
	before := s.Stats()
	budget.FailAfter(0)
	if _, _, err := s.Allocate(); !errors.Is(err, arena.ErrInjectedFailure) || s.Stats() != before {
		t.Fatal("failed refill changed the slab", err, s.Stats())
	}
	budget.FailAfter(-1)
	slot, next, err := s.Allocate()
	if err != nil || next != previous+1 || used != budget.Used() || s.IsLive(slot, previous) {
		t.Fatal("refilled short page", slot, next, err)
	}
}
