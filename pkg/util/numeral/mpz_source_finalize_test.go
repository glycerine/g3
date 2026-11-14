package numeral

// Source destructor: mpz.cpp:142–161; member order mpz.h:108–119 and
// mpn.h:99 (t_ab,t_ms,v,u), small_object_allocator.cpp:39–53.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestIntegerSourceFinalizeCallerPhysicalLifetimes(t *testing.T) {
	for _, direct := range []bool{false, true} {
		budget := arena.NewBudget(1 << 24)
		rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64})
		if e != nil {
			t.Fatal(e)
		}
		m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}, Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64}})
		if e != nil {
			t.Fatal(e)
		}
		count := 6
		if direct {
			count = 60
		}
		digits := make([]uint32, count)
		digits[0], digits[count-1] = 7, 1
		var caller Int
		if e = m.SetLimbs(&caller, digits); e != nil {
			t.Fatal(e)
		}
		object, e := m.CellObject(caller.Cell)
		if e != nil {
			t.Fatal(e)
		}
		allocator := m.Allocator
		raw, offset, e := m.Allocator.RawLocation(object)
		if e != nil {
			t.Fatal(e)
		}
		if direct != (offset == 0) {
			t.Fatal("fixture physical allocation branch", direct, offset)
		}
		if e = m.SourceFinalize(); e != nil {
			t.Fatal(e)
		}
		if _, e = m.Int64(&caller); !errors.Is(e, arena.ErrClosed) {
			t.Fatal("destroyed manager remains usable", e)
		}
		if direct {
			if rt.Stats().LiveBlocks != 1 || rt.Stats().PhysicalBytes != 256 {
				t.Fatal("source destructor freed caller direct allocation", rt.Stats())
			}
			if capacity, e := rt.ReadUint32(raw, 4); e != nil || capacity != 60 {
				t.Fatal("surviving direct capacity", capacity, e)
			}
			if digit, e := sourceCellWord(rt, allocator, object, raw, 0, 8); e != nil || digit != 7 {
				t.Fatal("surviving direct digits", digit, e)
			}
		} else {
			if rt.Stats().LiveBlocks != 0 {
				t.Fatal("source pool chunks retained", rt.Stats())
			}
			if _, e = rt.ReadUint32(raw, offset+4); e == nil {
				t.Fatal("caller pooled payload survived destroyed chunk")
			}
		}
		retained := budget.Used()
		if m.HeaderBytes == 0 || m.Cells.Stats().OwnerBytes == 0 || retained <= rt.Stats().TotalBytes {
			t.Fatal("owning adapter lost charged cleanup metadata")
		}
		state := rt.State()
		if e = m.SourceFinalize(); e != nil || rt.State() != state || budget.Used() != retained {
			t.Fatal("source destructor repeated actions", e)
		}
		if e = m.CloseChecked(); e != nil {
			t.Fatal(e)
		}
		if rt.Stats().LiveBlocks != 0 {
			t.Fatal("owning cleanup omitted surviving caller", rt.Stats())
		}
		if e = rt.CheckOpen(); e != nil {
			t.Fatal("borrowed runtime closed", e)
		}
		if e = rt.Close(); e != nil || budget.Used() != 0 {
			t.Fatal("owning metadata cleanup", e, budget.Used())
		}
	}
}

func TestIntegerSourceFinalizeReverseNaturalUnderAllocationLimit(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64, ThreadLocal: true})
	if e != nil {
		t.Fatal(e)
	}
	m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}, Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}})
	if e != nil {
		t.Fatal(e)
	}
	private := []NumericCellRef{m.Two64.Cell, m.IntMin.Cell, m.Tmp[0], m.Arg[0], m.Tmp[1], m.Arg[1]}
	var caller Int
	digits := make([]uint32, 60)
	digits[0], digits[59] = 7, 1
	if e = m.SetLimbs(&caller, digits); e != nil {
		t.Fatal(e)
	}
	natural := m.Natural
	for i := range natural.Division {
		if e = natural.ResizeDiv(i, 40000); e != nil {
			t.Fatal(e)
		}
	}
	// Source destruction releases native scratch even with Runtime limits armed.
	if e = rt.SetMaxCount(1); e != nil {
		t.Fatal(e)
	}
	if e = m.SourceFinalize(); e != nil {
		t.Fatal("allocation limit incorrectly applied to source destruction", e)
	}
	for i, ref := range private {
		if ValidNumericRef(m.Cells, ref) {
			t.Fatal("source private cell survived destruction", i)
		}
	}
	for i := range natural.Division {
		if natural.Division[i] != nil {
			t.Fatal("scratch retained", i)
		}
	}
	if rt.Stats().LiveBlocks != 1 || rt.State().OutOfMemory {
		t.Fatal("source destruction lost caller or fabricated OOM", rt.Stats(), rt.State())
	}
	if _, e = m.Int64(&caller); !errors.Is(e, arena.ErrClosed) {
		t.Fatal("destroyed source owner remains ordinary service", e)
	}
	if e = rt.SetMaxCount(0); e != nil {
		t.Fatal(e)
	}
	if e = m.CloseChecked(); e != nil {
		t.Fatal(e)
	}
	if e = rt.Close(); e != nil || budget.Used() != 0 {
		t.Fatal("owning final cleanup", e, budget.Used())
	}
}

// A real source diagnostic action can fail after allocator chunk destruction.
func TestIntegerSourceFinalizeDiagnosticFailureRetainsCaller(t *testing.T) {
	for _, retryFinalize := range []bool{false, true} {
		budget := arena.NewBudget(1 << 24)
		rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: budget, SizeTBits: 64})
		if e != nil {
			t.Fatal(e)
		}
		m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}, Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true, Windows: true}})
		if e != nil {
			t.Fatal(e)
		}
		var caller Int
		digits := make([]uint32, 60)
		digits[0], digits[59] = 7, 1
		if e = m.SetLimbs(&caller, digits); e != nil {
			t.Fatal(e)
		}
		raw, object, allocator := m.Cell(caller.Cell).Backing.Raw, m.Cell(caller.Cell).Backing.Object, m.Allocator
		if e = m.SourceFinalize(); !errors.Is(e, umemory.ErrAllocatorIO) {
			t.Fatal("actual source diagnostic refusal", e)
		}
		if m.CleanupCauseField != e || m.HeaderBytes == 0 || m.Cells.Stats().Live != 1 || (m.Natural == nil || !m.Natural.sourceFinalized || m.Natural.SourceStage != 4 || m.Natural.HeaderBytes == 0) || m.Tmp != [2]NumericCellRef{} || m.Arg != [2]NumericCellRef{} || m.Two64.Cell != 0 || m.IntMin.Cell != 0 {
			t.Fatal("completed source destructor prefix lost", e)
		}
		if rt.Stats().LiveBlocks != 1 {
			t.Fatal("source diagnostic failure freed caller", rt.Stats())
		}
		if v, e := sourceCellWord(rt, allocator, object, raw, 0, 8); e != nil || v != 7 {
			t.Fatal("surviving caller prefix", v, e)
		}
		if retryFinalize {
			state := rt.State()
			used := budget.Used()
			if e = m.SourceFinalize(); e != nil || rt.State() != state || budget.Used() != used || m.CleanupCauseField != nil {
				t.Fatal("source diagnostic prefix replayed", e)
			}
		}

		if e = m.CloseChecked(); e != nil {
			t.Fatal("owning direct cleanup", e)
		}
		if e = rt.Close(); e != nil || budget.Used() != 0 {
			t.Fatal("retained owner cleanup", e, budget.Used())
		}

	}
}

// ensure_tmp frees its old cell before a refused replacement. The resulting
// missing pointer is source-undefined at destructor capacity access; owning
// cleanup is a separate adapter and must preserve the stopped source history.
func TestIntegerSourceFinalizeMissingScratchOwningAbort(t *testing.T) {
	m, b := mpzPublicationDirectOwner(t)
	old := m.Tmp[0]
	b.FailAfter(0)
	e := m.EnsureTmp(0, 33)
	if !numericPhysicalReservationRefusal(e) || m.Tmp[0] != 0 || ValidNumericRef(m.Cells, old) {
		t.Fatal("actual free-before-replacement prefix", e)
	}
	b.FailAfter(-1)
	remaining := [3]NumericCellRef{m.Arg[0], m.Tmp[1], m.Arg[1]}
	if e = m.SourceFinalize(); !errors.Is(e, arena.ErrInvalidSlot) {
		t.Fatal("missing scratch destructor access", e)
	}
	if !m.sourceGuard || m.SourceFinalizeStage != 2 || m.SourceFinalized || m.Two64.Cell != 0 || m.IntMin.Cell != 0 || m.HeaderBytes == 0 {
		t.Fatal("source guard lost completed prefix")
	}
	for _, r := range remaining {
		if !ValidNumericRef(m.Cells, r) {
			t.Fatal("later source root consumed", r)
		}
	}
	state, used := m.Memory.State(), b.Used()
	if e = m.SourceFinalize(); !errors.Is(e, arena.ErrInvalidSlot) || m.Memory.State() != state || b.Used() != used {
		t.Fatal("guard retry repeated completed source actions", e)
	}
	if e = m.CloseChecked(); e != nil {
		t.Fatal("explicit owning abort cleanup", e)
	}
	if !m.sourceGuard || m.SourceFinalizeStage != 2 || m.SourceFinalized || !m.closed || b.Used() != 0 {
		t.Fatal("owning cleanup falsely completed source destructor", b.Used())
	}
}
