package numeral

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestIntegerSourceCellBackingInitializesCapacityOnly(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	a, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: r})
	if e != nil {
		t.Fatal(e)
	}
	c, e := allocateSourceCell(a, 6)
	if e != nil {
		t.Fatal(e)
	}
	if c.Requested != 32 || c.extent != 32 || c.Object == (umemory.ObjectRef{}) || r.Stats().PhysicalBytes != 8200 {
		t.Fatal("actual source cell backing topology")
	}
	if v, e := c.ReadWord(a, 4); e != nil || v != 6 {
		t.Fatal("actual capacity store", v, e)
	}
	if _, e := c.ReadWord(a, 0); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("invented m_size initialization: %v", e)
	}
	// Digits are the object's native payload (z3's plain digit array): one
	// word per digit in the rounded extent, never written to the byte runtime.
	if d, e := a.Words(c.Object); e != nil || len(d) != 6 {
		t.Fatal("native digit payload", len(d), e)
	}
	if e = c.writeWord(a, 8, 99); e != nil {
		t.Fatal(e)
	}
	if v, e := c.ReadWord(a, 8); e != nil || v != 99 {
		t.Fatal("actual digit backing", v, e)
	}
	if v, e := c.ReadWord(a, 28); e != nil || v != 0 {
		t.Fatal("last digit in rounded extent", v, e)
	}
	if _, e := r.ReadUint32(c.Raw, c.Offset+8); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatal("digit routed through the byte runtime", e)
	}
	if _, e := c.ReadWord(a, 32); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("cell access escaped rounded extent", e)
	}
	if e = a.Deallocate(c.Requested, c.Object); e != nil {
		t.Fatal(e)
	}
	if _, e := c.ReadWord(a, 8); e == nil {
		t.Fatal("freed generation readable through raw chunk")
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatal("source cell owner leak")
	}
}

func TestIntegerSourceCellWrappedHeaderGuardUsesActualRoundedExtent(t *testing.T) {
	for _, bits := range []uint8{32, 64} {
		b := arena.NewBudget(1 << 24)
		r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: bits})
		if e != nil {
			t.Fatal(e)
		}
		a, e := umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: r})
		if e != nil {
			t.Fatal(e)
		}
		// cell_size wraps to4 even on LP64. LP64 pool rounds to8; ILP32
		// pool gives4, so the capacity-header write is the actual endangered action.
		c, e := allocateSourceCell(a, 0x3fffffff)
		if bits == 32 {
			if !errors.Is(e, arena.ErrBounds) {
				t.Fatalf("ILP32 capacity action %v", e)
			}
		} else {
			if e != nil {
				t.Fatal(e)
			}
			if v, e := c.ReadWord(a, 4); e != nil || v != 0x3fffffff {
				t.Fatal("LP64 rounded header write", v, e)
			}
		}
		if c.Object == (umemory.ObjectRef{}) || c.Requested != 4 || c.extent != uint64(bits/8) || r.Stats().LiveBlocks != 1 {
			t.Fatal("wrapped header guard lost allocated object")
		}
		if e = c.writeWord(a, 8, 1); !errors.Is(e, arena.ErrBounds) {
			t.Fatal("digit escaped actual object", e)
		}
		if e = a.Deallocate(c.Requested, c.Object); e != nil {
			t.Fatal(e)
		}
		if e = a.Close(); e != nil {
			t.Fatal(e)
		}
		if e = r.Close(); e != nil {
			t.Fatal(e)
		}
		if b.Used() != 0 {
			t.Fatal("wrapped cell cleanup leak")
		}
	}
}

// sourceCellWord reads the source cell word at a byte offset the way the
// cell stores it: the m_size/m_capacity header from the memory runtime at
// base+offset, and digits from the object's native word payload. It does not
// validate the object, so it can inspect a caller cell that survived the
// source allocator's destructor.
func sourceCellWord(rt *umemory.Runtime, a *umemory.SmallAllocator, object umemory.ObjectRef, raw umemory.Block, base, offset uint64) (uint32, error) {
	if offset < sourceCellDigitOffset {
		return rt.ReadUint32(raw, base+offset)
	}
	d := a.WordsUnchecked(object)
	i := (offset - sourceCellDigitOffset) / 4
	if i >= uint64(len(d)) {
		return 0, arena.ErrBounds
	}
	return d[i], nil
}
