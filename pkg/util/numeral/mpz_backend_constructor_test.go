package numeral

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func TestIntegerMandatoryCellConstructorTopology(t *testing.T) {
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
	m.Close()
	if e = rt.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatalf("cold root leak %d", b.Used())
	}
}

func TestIntegerMandatoryDefaultRuntimeOwnership(t *testing.T) {
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
