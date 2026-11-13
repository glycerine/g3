package memory

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestMemoryUint32CodecActualBackingAndInitializedBytes(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	x, e := r.Allocate(8)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.ReadUint32(x, 0); !errors.Is(e, ErrRuntimeUninitialized) {
		t.Fatalf("uninitialized word: %v", e)
	}
	if e = r.Write(x, 0, 0x12); e != nil {
		t.Fatal(e)
	}
	if _, e = r.ReadUint32(x, 0); !errors.Is(e, ErrRuntimeUninitialized) {
		t.Fatalf("partially initialized word: %v", e)
	}
	before := r.State()
	if e = r.WriteUint32(x, 0, 0x78563412); e != nil {
		t.Fatal(e)
	}
	for i, w := range []byte{0x12, 0x34, 0x56, 0x78} {
		v, e := r.Read(x, uint64(i))
		if e != nil || v != w {
			t.Fatalf("physical byte %d: %x %v", i, v, e)
		}
	}
	v, e := r.ReadUint32(x, 0)
	if e != nil || v != 0x78563412 {
		t.Fatalf("word: %x %v", v, e)
	}
	if r.State() != before {
		t.Fatal("codec invented allocation event")
	}
	if e = r.WriteUint32(x, 5, 1); !errors.Is(e, arena.ErrBounds) {
		t.Fatalf("endangered write: %v", e)
	}
	for _, offset := range []uint64{5, math.MaxUint64} {
		if _, e = r.ReadUint32(x, offset); !errors.Is(e, arena.ErrBounds) {
			t.Fatalf("offset %d: %v", offset, e)
		}
	}
	if e = r.Deallocate(&x); e != nil {
		t.Fatal(e)
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatalf("leaked charge %d", b.Used())
	}
}

func TestRuntimeUint32AcrossInitializationBitmapBoundary(t *testing.T) {
	b := arena.NewBudget(1 << 16)
	r, err := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Allocate(64)
	if err != nil {
		t.Fatal(err)
	}
	// Payload offset 53 starts at physical byte 61 and crosses bit 63.
	for i := uint64(0); i < 3; i++ {
		if err := r.Write(h, 53+i, byte(i+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := r.ReadUint32(h, 53); !errors.Is(err, ErrRuntimeUninitialized) {
			t.Fatal("partly initialized crossing word", i, err)
		}
	}
	if err := r.Write(h, 56, 4); err != nil {
		t.Fatal(err)
	}
	if v, err := r.ReadUint32(h, 53); err != nil || v != 0x04030201 {
		t.Fatal("crossing read", v, err)
	}
	if err := r.WriteUint32(h, 53, 0x78563412); err != nil {
		t.Fatal(err)
	}
	for i, want := range []byte{0x12, 0x34, 0x56, 0x78} {
		if got, err := r.Read(h, 53+uint64(i)); err != nil || got != want {
			t.Fatal("crossing write", i, got, want, err)
		}
	}
	if err := r.Deallocate(&h); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil || b.Used() != 0 {
		t.Fatal("crossing cleanup", err, b.Used())
	}
}

func TestMemoryAllocatorUint32CodecUsesIntrusivePayload(t *testing.T) {
	b := arena.NewBudget(1 << 24)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r})
	if e != nil {
		t.Fatal(e)
	}
	x, e := a.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.WriteUint32(x, 4, 6); e != nil {
		t.Fatal(e)
	}
	raw, off, e := a.RawLocation(x)
	if e != nil {
		t.Fatal(e)
	}
	v, e := r.ReadUint32(raw, off+4)
	if e != nil || v != 6 {
		t.Fatalf("not actual shared capacity bytes %d %v", v, e)
	}
	state := r.State()
	if e = a.Deallocate(32, x); e != nil {
		t.Fatal(e)
	}
	y, e := a.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	if y.LogicalAddress() != x.LogicalAddress() || y.Generation() == x.Generation() {
		t.Fatal("source reuse/checked lifetime mismatch")
	}
	if _, e = a.ReadUint32(x, 4); e == nil {
		t.Fatal("stale generation accepted")
	}
	if e = a.WriteUint32(y, 4, 6); e != nil {
		t.Fatal(e)
	}
	if r.State() != state {
		t.Fatal("recycle/codec invented raw event")
	}
	if e = a.Deallocate(32, y); e != nil {
		t.Fatal(e)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if b.Used() != 0 {
		t.Fatal("leaked metadata")
	}
}

func TestMemoryMPZRoundedPhysicalObjectExtent(t *testing.T) {
	for _, tc := range []struct {
		bits   uint8
		debug  bool
		extent uint64
	}{{64, false, 8}, {32, false, 4}, {64, true, 4}} {
		b := arena.NewBudget(1 << 20)
		r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: tc.bits})
		if e != nil {
			t.Fatal(e)
		}
		a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r, Profile: SmallAllocatorProfile{PointerBits: tc.bits, Debug: tc.debug}})
		if e != nil {
			t.Fatal(e)
		}
		x, e := a.Allocate(4)
		if e != nil {
			t.Fatal(e)
		}
		extent, e := a.ObjectExtent(x)
		if e != nil || extent != tc.extent {
			t.Fatalf("actual rounded extent%d want%d %v", extent, tc.extent, e)
		}
		if e = a.Write(x, 4, 1); !errors.Is(e, arena.ErrBounds) {
			t.Fatal("ordinary requested-byte API widened")
		}
		raw, off, e := a.RawLocation(x)
		if e != nil {
			t.Fatal(e)
		}
		if extent >= 8 { // Real LP64 pooled cell_size wrap still fits both header words.
			if e = r.WriteUint32(raw, off+4, 0xffffffff); e != nil {
				t.Fatal(e)
			}
			v, e := r.ReadUint32(raw, off+4)
			if e != nil || v != 0xffffffff {
				t.Fatal("actual physical header publication lost")
			}
		}
		if e = a.Deallocate(4, x); e != nil {
			t.Fatal(e)
		}
		if e = a.Close(); e != nil {
			t.Fatal(e)
		}
		if e = r.Close(); e != nil {
			t.Fatal(e)
		}
		if b.Used() != 0 {
			t.Fatal("root leak")
		}
	}
}

func TestMemoryUint32CodecGuardsAndZeroAlloc(t *testing.T) {
	b := arena.NewBudget(1 << 20)
	r, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	other, e := NewRuntime(RuntimeConfig{Budget: b, SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: r})
	if e != nil {
		t.Fatal(e)
	}
	foreign, e := NewSmallAllocator(SmallAllocatorConfig{Runtime: other})
	if e != nil {
		t.Fatal(e)
	}
	x, e := a.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	raw, off, e := a.RawLocation(x)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.WriteUint32(x, 1, 0x11223344); e != nil {
		t.Fatal(e)
	}
	if v, e := r.ReadUint32(raw, off+1); e != nil || v != 0x11223344 {
		t.Fatal("unaligned concrete byte codec mismatch")
	}
	if _, e = other.ReadUint32(raw, off); !errors.Is(e, ErrRuntimeOwner) {
		t.Fatalf("foreign runtime %v", e)
	}
	if _, e = foreign.ObjectExtent(x); !errors.Is(e, ErrAllocatorOwner) {
		t.Fatalf("foreign allocator %v", e)
	}
	copiedRuntime := *r
	if e = copiedRuntime.WriteUint32(raw, off, 1); !errors.Is(e, ErrRuntimeOwner) {
		t.Fatalf("copied runtime %v", e)
	}
	copiedAllocator := *a
	if _, e = copiedAllocator.ObjectExtent(x); !errors.Is(e, ErrAllocatorOwner) {
		t.Fatalf("copied allocator %v", e)
	}
	var nilRuntime *Runtime
	var nilAllocator *SmallAllocator
	if _, e = nilRuntime.ReadUint32(raw, 0); !errors.Is(e, ErrRuntimeOwner) {
		t.Fatalf("nil runtime %v", e)
	}
	if e = nilAllocator.WriteUint32(x, 0, 1); !errors.Is(e, ErrAllocatorOwner) {
		t.Fatalf("nil allocator %v", e)
	}
	if e = a.WriteUint32(x, 28, 0xaabbccdd); e != nil {
		t.Fatal(e)
	}
	if e = a.WriteUint32(x, 29, 0); !errors.Is(e, arena.ErrBounds) {
		t.Fatal("endangered word published partial bytes")
	}
	if v, e := a.ReadUint32(x, 28); e != nil || v != 0xaabbccdd {
		t.Fatal("failed word changed previous value")
	}
	state, stats, used := r.State(), r.Stats(), b.Used()
	b.FailAfter(0)
	var problem error
	var wrongValue bool
	allocs := testing.AllocsPerRun(1000, func() {
		if problem != nil || wrongValue {
			return
		}
		if e := a.WriteUint32(x, 0, 7); e != nil {
			problem = e
			return
		}
		if v, e := a.ReadUint32(x, 0); e != nil || v != 7 {
			problem = e
			wrongValue = v != 7
			return
		}
		if _, e := a.ObjectExtent(x); e != nil {
			problem = e
			return
		}
		if e := r.WriteUint32(raw, off+4, 8); e != nil {
			problem = e
			return
		}
		if v, e := r.ReadUint32(raw, off+4); e != nil || v != 8 {
			problem = e
			wrongValue = v != 8
		}
	})
	if problem != nil || wrongValue || allocs != 0 || r.State() != state || r.Stats() != stats || b.Used() != used {
		t.Fatalf("warm codec hostalloc=%g err%v", allocs, problem)
	}
	b.FailAfter(-1)
	if e = a.Deallocate(32, x); e != nil {
		t.Fatal(e)
	}
	if _, e = a.ObjectExtent(x); !errors.Is(e, ErrAllocatorOwner) {
		t.Fatalf("freed lifetime extent %v", e)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = a.ReadUint32(x, 0); !errors.Is(e, arena.ErrClosed) {
		t.Fatalf("closed allocator %v", e)
	}
	if e = foreign.Close(); e != nil {
		t.Fatal(e)
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if e = other.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = r.ReadUint32(raw, 0); !errors.Is(e, arena.ErrClosed) {
		t.Fatalf("closed runtime %v", e)
	}
	if b.Used() != 0 {
		t.Fatal("owners leaked")
	}
}

// CopyUint32s copies the same words WriteUint32/ReadUint32 would, marks the
// destination initialized, and rejects out-of-range and uninitialized source
// bytes like the per-word accessors.
func TestMemoryCopyUint32sMatchesWordAccess(t *testing.T) {
	r, e := NewRuntime(RuntimeConfig{Budget: arena.NewBudget(1 << 24), SizeTBits: 64})
	if e != nil {
		t.Fatal(e)
	}
	src, e := r.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	dst, e := r.Allocate(32)
	if e != nil {
		t.Fatal(e)
	}
	for i := uint64(0); i < 6; i++ {
		if e = r.WriteUint32(src, 4*i, uint32(0x01020304*(i+1))); e != nil {
			t.Fatal(e)
		}
	}
	if e = r.CopyUint32s(dst, 8, src, 4, 5); e != nil {
		t.Fatal(e)
	}
	for i := uint64(0); i < 5; i++ {
		want, _ := r.ReadUint32(src, 4+4*i)
		got, e := r.ReadUint32(dst, 8+4*i)
		if e != nil || got != want {
			t.Fatalf("word %d: got %x (%v), want %x", i, got, e, want)
		}
	}
	if _, e = r.ReadUint32(dst, 0); !errors.Is(e, ErrRuntimeUninitialized) {
		t.Fatalf("untouched destination word: %v", e)
	}
	if e = r.CopyUint32s(dst, 20, src, 0, 4); !errors.Is(e, arena.ErrBounds) {
		t.Fatalf("destination overrun: %v", e)
	}
	if e = r.CopyUint32s(dst, 0, src, 16, 5); !errors.Is(e, arena.ErrBounds) {
		t.Fatalf("source overrun: %v", e)
	}
	if e = r.CopyUint32s(src, 0, dst, 0, 1); !errors.Is(e, ErrRuntimeUninitialized) {
		t.Fatalf("uninitialized source: %v", e)
	}
	if e = r.CopyUint32s(dst, 0, src, 0, 0); e != nil {
		t.Fatalf("empty copy: %v", e)
	}
}

// initializedRangeSet and markInitializedRange agree with the per-byte bit
// loop they replace, including ranges that cross bitmap words.
func TestInitializedRangeHelpersMatchPerByte(t *testing.T) {
	for from := uint64(0); from < 140; from++ {
		for n := uint64(0); from+n <= 200; n++ {
			bits := make([]uint64, 4)
			markInitializedRange(bits, from, n)
			for i := uint64(0); i < 256; i++ {
				want := i >= from && i < from+n
				if got := bits[i/64]&(uint64(1)<<(i%64)) != 0; got != want {
					t.Fatalf("mark(%d,%d): bit %d = %v", from, n, i, got)
				}
			}
			if !initializedRangeSet(bits, from, n) {
				t.Fatalf("range(%d,%d) not set after marking", from, n)
			}
			if n > 0 {
				hole := from + n/2
				bits[hole/64] &^= uint64(1) << (hole % 64)
				if initializedRangeSet(bits, from, n) {
					t.Fatalf("range(%d,%d) missed hole at %d", from, n, hole)
				}
			}
		}
	}
}
