package numeral

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Cold API method-set contracts: the charged owner itself retains failure.
// No separately allocated wrapper is accepted by this proposed surface.
var _ umemory.ConstructionFailure = (*IntegerManager)(nil)
var _ umemory.ConstructionFailure = (*RationalManager)(nil)
var _ umemory.ConstructionFailure = (*RationalService)(nil)

func phaseBCheckAcyclicFailure(t *testing.T, e error) {
	t.Helper()
	for depth := 0; e != nil; depth++ {
		if depth > 4 {
			t.Fatal("retained owner failure chain is cyclic or deeper than its actual constructor graph")
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return
		}
		next := u.Unwrap()
		if next == e {
			t.Fatal("owner failure unwraps to itself")
		}
		e = next
	}
}

func TestIntegerDefaultActualAllocatorConstructorTopology(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Memory: rt, Profile: umemory.SmallAllocatorProfile{PointerBits: 64}})
	if e != nil {
		t.Fatal(e)
	}
	if m.RuntimeOwner() != rt {
		t.Fatal("different runtime or idle backend")
	}
	s := rt.Stats()
	n, e := rt.AllocationCount()
	if e != nil {
		t.Fatal(e)
	}
	if s.LiveBlocks != 1 || s.PhysicalBytes != 8200 || n != 1 {
		t.Fatalf("constructor physical topology %+v count%d", s, n)
	}
	refs := []NumericCellRef{m.Tmp[0], m.Arg[0], m.Tmp[1], m.Arg[1], m.IntMin.Cell, m.Two64.Cell}
	objects := make([]umemory.ObjectRef, len(refs))
	for i, ref := range refs {
		objects[i], e = m.CellObject(ref)
		if e != nil {
			t.Fatal(e)
		}
		raw, off, e := m.Allocator.RawLocation(objects[i])
		if e != nil {
			t.Fatal(e)
		}
		cap, e := rt.ReadUint32(raw, off+4)
		if e != nil || cap != 6 {
			t.Fatalf("capacity%d: %d %v", i, cap, e)
		}
	}
	// Constructor allocation order is tmp0,arg0,tmp1,arg1,intMin,two64;
	// Add exchanges the original tmp0 and two64 objects without reallocating.
	if objects[5].LogicalAddress() >= objects[1].LogicalAddress() || objects[0].LogicalAddress() <= objects[4].LogicalAddress() {
		t.Fatal("source final two64/tmp0 exchange lost")
	}
	for _, i := range []int{1, 3} {
		raw, off, _ := m.Allocator.RawLocation(objects[i])
		v, e := rt.ReadUint32(raw, off)
		if e != nil || v != 1 {
			t.Fatalf("arg source size %d %v", v, e)
		}
	}
	// Only actual source writes establish initialized physical fields.
	rawTmp1, offTmp1, _ := m.Allocator.RawLocation(objects[2])
	if _, e = rt.ReadUint32(rawTmp1, offTmp1); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("untouched tmp1 size synthesized: %v", e)
	}
	rawArg0, offArg0, _ := m.Allocator.RawLocation(objects[1])
	// Digits live in the native payload; the runtime digit bytes stay untouched.
	if _, e = rt.ReadUint32(rawArg0, offArg0+8); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("digit written through the byte runtime: %v", e)
	}
	rawArg1, offArg1, _ := m.Allocator.RawLocation(objects[3])
	if v, e := sourceCellWord(rt, m.Allocator, objects[3], rawArg1, offArg1, 8); e != nil || v != 1 {
		t.Fatalf("source get_sign_cell<1> one digit %d %v", v, e)
	}
	for _, tc := range []struct {
		index  int
		digits []uint32
	}{{0, []uint32{0xffffffff, 0xffffffff}}, {4, []uint32{0x80000000}}, {5, []uint32{0, 0, 1}}} {
		raw, off, _ := m.Allocator.RawLocation(objects[tc.index])
		n, e := rt.ReadUint32(raw, off)
		if e != nil || n != uint32(len(tc.digits)) {
			t.Fatalf("source size%d=%d %v", tc.index, n, e)
		}
		for i, w := range tc.digits {
			v, e := sourceCellWord(rt, m.Allocator, objects[tc.index], raw, off, 8+4*uint64(i))
			if e != nil || v != w {
				t.Fatalf("source object%d digit%d=%x want%x %v", tc.index, i, v, w, e)
			}
		}
	}
	// The 32-bit-digit signed64 setter initializes its high digit even
	// when logical size is one (mpz.cpp:184-189). This spare word is real.
	rawMin, offMin, _ := m.Allocator.RawLocation(objects[4])
	if v, e := sourceCellWord(rt, m.Allocator, objects[4], rawMin, offMin, 12); e != nil || v != 0 {
		t.Fatalf("source intMin initialized high word %x %v", v, e)
	}
	if e = m.SourceFinalize(); e != nil {
		t.Fatal(e)
	}
	if rt.Stats().LiveBlocks != 0 {
		t.Fatal("pooled member chunks survived source finalization")
	}
	m.Close()
	if e = rt.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatalf("cold root leak %d", b.Used())
	}
}

func TestIntegerDefaultRuntimeIsActuallyOwnedBackend(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	if e != nil {
		t.Fatal(e)
	}
	r := m.RuntimeOwner()
	if r == nil || r.Stats().LiveBlocks == 0 {
		t.Fatal("default still uses disconnected slab/Words payload")
	}
	m.Close()
	if r.Stats().TotalBytes != 0 || b.Used() != 0 {
		t.Fatal("default owning cleanup omitted runtime")
	}
}

func TestIntegerSourceFinalizeRetainsActualDirectCallerRoot(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Memory: rt})
	if e != nil {
		t.Fatal(e)
	}
	x := NewInt(0)
	digits := make([]uint32, 60)
	digits[0] = 7
	digits[59] = 1
	if e = m.SetLimbs(&x, digits); e != nil {
		t.Fatal(e)
	}
	o, e := m.CellObject(x.Cell)
	if e != nil {
		t.Fatal(e)
	}
	raw, off, e := m.Allocator.RawLocation(o)
	if e != nil || off != 0 {
		t.Fatalf("expected real direct cell %d %v", off, e)
	}
	if e = m.SourceFinalize(); e != nil {
		t.Fatal(e)
	}
	if rt.Stats().LiveBlocks != 1 || rt.Stats().PhysicalBytes != 256 {
		t.Fatalf("source destructor invented caller direct free %+v", rt.Stats())
	}
	cap, e := rt.ReadUint32(raw, 4)
	if e != nil || cap != 60 {
		t.Fatalf("caller capacity bytes lost %d %v", cap, e)
	}
	if e = m.CloseChecked(); e != nil {
		t.Fatal(e)
	}
	if rt.Stats().LiveBlocks != 0 {
		t.Fatal("owning Go cleanup omitted surviving direct root")
	}
	if e = rt.CheckOpen(); e != nil {
		t.Fatal("borrowed runtime incorrectly closed")
	}
	if e = rt.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatal("charged cold roots leaked")
	}
}

func TestIntegerSourceCellSizeUnsignedDomain(t *testing.T) {
	for _, tc := range []struct {
		cap   uint32
		bytes uint32
	}{{0, 8}, {6, 32}, {59, 244}, {60, 248}, {61, 252}, {0x3ffffffe, 0}, {0x3fffffff, 4}, {0x40000000, 8}, {0xffffffff, 4}} {
		if got := sourceMPZCellBytes(tc.cap); got != uint64(tc.bytes) {
			t.Fatalf("capacity%x source unsigned extent%d expected%d", tc.cap, got, tc.bytes)
		}
	}
}

func TestIntegerConstructorRefusalRetainsTransitiveOwnership(t *testing.T) {
	for kind := 0; kind < 3; kind++ {
		success := false
		for stop := int64(0); stop < 64; stop++ {
			b := arena.NewBudget(1 << 24)
			r, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
			if e != nil {
				t.Fatal(e)
			}
			baseline := b.Used()
			b.FailAfter(stop)
			c := IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Memory: r}
			var construction error
			switch kind {
			case 0:
				m, e := NewIntegerManager(c)
				construction = e
				if e == nil {
					if e = m.CloseChecked(); e != nil {
						t.Fatal(e)
					}
				}
			case 1:
				m, e := NewRationalManager(RationalConfig{Integers: c})
				construction = e
				if e == nil {
					if e = m.CloseChecked(); e != nil {
						t.Fatal(e)
					}
				}
			case 2:
				m, e := NewRationalService(RationalServiceConfig{Integers: c})
				construction = e
				if e == nil {
					if e = m.Close(); e != nil {
						t.Fatal(e)
					}
				}
			}
			if construction != nil {
				phaseBCheckAcyclicFailure(t, construction)
				if !numericPhysicalReservationRefusal(construction) {
					t.Fatalf("kind%d stop%d error %v", kind, stop, construction)
				}
				var partial umemory.ConstructionFailure
				if errors.As(construction, &partial) {
					original := partial.OriginalCause()
					if original == nil || partial.Unwrap() != original {
						t.Fatal("retained owner lost original cause or acyclic unwrap")
					}
					if partial.RuntimeOwner() != r || b.Used() <= baseline {
						t.Fatal("cleanup refusal lost charged owner")
					}
					b.FailAfter(-1)
					if e = partial.Cleanup(); e != nil {
						t.Fatal(e)
					}
					if partial.OriginalCause() != original || partial.CleanupCause() != nil || partial.RuntimeOwner() != r {
						t.Fatal("cleanup rewrote original cause/domain or retained stale cleanup error")
					}
					phaseBCheckAcyclicFailure(t, partial)
				}
			} else {
				success = true
			}
			if r.Stats().LiveBlocks != 0 || b.Used() != r.Stats().TotalBytes {
				t.Fatalf("kind%d stop%d immediate unowned roots %+v used%d baseline%d", kind, stop, r.Stats(), b.Used(), baseline)
			}
			if e = r.CheckOpen(); e != nil {
				t.Fatal("failed or successful subowner closed borrowed runtime")
			}
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
			if b.Used() != 0 {
				t.Fatal("runtime metadata leak")
			}
			if success {
				break
			}
		}
		if !success {
			t.Fatalf("kind%d no successful constructor within bounded reservation sweep", kind)
		}
	}
}

// Source ensure_tmp/borrow prepares capacity, not a physical m_size write.
func TestIntegerScratchSpanDoesNotPublishPhysicalSize(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	rt, e := umemory.NewRuntime(umemory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	m, e := NewIntegerManager(IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}, Memory: rt})
	if e != nil {
		t.Fatal(e)
	}
	object, e := m.CellObject(m.Tmp[1])
	if e != nil {
		t.Fatal(e)
	}
	raw, off, e := m.Allocator.RawLocation(object)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.prepareTmpSpan(1, 4); e != nil {
		t.Fatal(e)
	}
	if _, e = rt.ReadUint32(raw, off); !errors.Is(e, umemory.ErrRuntimeUninitialized) {
		t.Fatalf("scratch span invented physical size: %v", e)
	}
	// A typed64 setter writes low+high first, and publishes size last.
	x := NewInt(0)
	if e = m.SetUint64(&x, 0x80000000); e != nil {
		t.Fatal(e)
	}
	object, e = m.CellObject(x.Cell)
	if e != nil {
		t.Fatal(e)
	}
	raw, off, e = m.Allocator.RawLocation(object)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		offset uint64
		value  uint32
	}{{0, 1}, {8, 0x80000000}, {12, 0}} {
		if v, e := sourceCellWord(rt, m.Allocator, object, raw, off, tc.offset); e != nil || v != tc.value {
			t.Fatalf("setter offset%d got%x %v", tc.offset, v, e)
		}
	}
	if e = m.Del(&x); e != nil {
		t.Fatal(e)
	}
	if e = m.CloseChecked(); e != nil {
		t.Fatal(e)
	}
	if e = rt.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatal("scratch header fixture leaked physical roots")
	}
}
