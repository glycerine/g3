package arena

import (
	"errors"
	"fmt"
	"testing"
)

func TestSmallBytesReadUint32LE(t *testing.T) {
	for _, size := range []uint32{7, 512} {
		a := newSmallTest(t, 80, nil)
		r := smallAlloc(t, a, size)
		for i := uint32(0); i < size; i++ {
			if err := a.Write(r, i, byte(i*37+11)); err != nil {
				t.Fatal(err)
			}
		}
		for offset := uint32(0); offset+4 <= size; offset++ {
			var want uint32
			for i := uint32(0); i < 4; i++ {
				b, err := a.Read(r, offset+i)
				if err != nil {
					t.Fatal(err)
				}
				want |= uint32(b) << (8 * i)
			}
			got, err := a.ReadUint32LE(r, offset)
			if err != nil || got != want {
				t.Fatalf("size %d offset %d: %x want %x: %v", size, offset, got, want, err)
			}
		}
		for _, offset := range []uint32{size - 3, size, ^uint32(0)} {
			if _, err := a.ReadUint32LE(r, offset); !errors.Is(err, ErrBounds) {
				t.Fatal("bounds", offset, err)
			}
		}
		smallFree(t, a, size, r)
		replacement := smallAlloc(t, a, size)
		if _, err := a.ReadUint32LE(r, 0); !errors.Is(err, ErrInvalidSlot) {
			t.Fatal("stale generation accepted", err)
		}
		smallFree(t, a, size, replacement)
		a.Close()
		if _, err := a.ReadUint32LE(replacement, 0); !errors.Is(err, ErrClosed) {
			t.Fatal("closed arena accepted", err)
		}
	}
}

func TestSmallBytesReadUint32LEsMatchesSingleReads(t *testing.T) {
	store := newSmallTest(t, 80, nil)
	ref := smallAlloc(t, store, 40)
	var err error
	for i := uint32(0); i < 10; i++ {
		if err = store.WriteUint32LE(ref, i*4, 1000+i); err != nil {
			t.Fatal(err)
		}
	}
	block := make([]uint32, 4)
	if err = store.ReadUint32LEs(ref, 8, block); err != nil {
		t.Fatal(err)
	}
	for i, got := range block {
		single, err := store.ReadUint32LE(ref, 8+uint32(i)*4)
		if err != nil || single != got || got != 1002+uint32(i) {
			t.Fatal("block read", i, got, single, err)
		}
	}
	// One word past the end is refused without a partial read mattering.
	if err = store.ReadUint32LEs(ref, 36, make([]uint32, 2)); err != ErrBounds {
		t.Fatal("overrun", err)
	}
	if err = store.ReadUint32LEs(ref, 40, nil); err != nil {
		t.Fatal("empty read at the end", err)
	}
	if err = store.ReadUint32LEs(ref, ^uint32(0), make([]uint32, 2)); err != ErrBounds {
		t.Fatal("offset overflow", err)
	}
	if err = store.WriteUint32LE(ref, 38, 1); err != ErrBounds {
		t.Fatal("write overrun", err)
	}
	if err = store.Deallocate(40, ref); err != nil {
		t.Fatal(err)
	}
	if err = store.ReadUint32LEs(ref, 0, block); err != ErrInvalidSlot {
		t.Fatal("stale reference", err)
	}
	if _, err = store.ReadUint32LE(ref, 0); err != ErrInvalidSlot {
		t.Fatal("stale single read", err)
	}
}

func TestSmallBytesWordReadsDoNotAllocate(t *testing.T) {
	store := newSmallTest(t, 80, nil)
	ref := smallAlloc(t, store, 16)
	var err error
	_ = err
	var words [2]uint32
	if allocs := testing.AllocsPerRun(100, func() {
		_, _ = store.ReadUint32LE(ref, 0)
		_ = store.ReadUint32LEs(ref, 0, words[:])
		_ = store.WriteUint32LE(ref, 4, 7)
	}); allocs != 0 {
		t.Fatalf("%v allocations", allocs)
	}
}

func TestSmallBytesBorrowSeesTheBlockInPlace(t *testing.T) {
	store := newSmallTest(t, 80, nil)
	ref := smallAlloc(t, store, 16)
	if err := store.WriteUint32LE(ref, 4, 0xAABBCCDD); err != nil {
		t.Fatal(err)
	}
	data, err := store.Borrow(ref)
	if err != nil || len(data) != 16 || cap(data) != 16 {
		t.Fatal("borrow", len(data), cap(data), err)
	}
	if data[4] != 0xDD || data[7] != 0xAA {
		t.Fatal("borrow does not read the block", data)
	}
	data[8] = 0x11
	if got, err := store.Read(ref, 8); err != nil || got != 0x11 {
		t.Fatal("borrow does not write the block", got, err)
	}
	if empty, err := store.Borrow(SmallBytesRef{}); err != nil || empty != nil {
		t.Fatal("null reference", empty, err)
	}
	if err = store.Deallocate(16, ref); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Borrow(ref); err != ErrInvalidSlot {
		t.Fatal("stale borrow", err)
	}
}

func TestSmallBytesViewsAcrossSlotReuseAndReset(t *testing.T) {
	for _, size := range []uint32{13, 300} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			a := newSmallTest(t, 80, nil)
			old := smallAlloc(t, a, size)
			check := func(ref SmallBytesRef, n int, wantErr error) {
				t.Helper()
				for _, access := range []func(SmallBytesRef) ([]byte, error){a.Borrow, a.View} {
					data, err := access(ref)
					if err != wantErr || len(data) != n || cap(data) != n {
						t.Fatalf("ref %v: length %d, capacity %d, error %v; want %d and %v", ref, len(data), cap(data), err, n, wantErr)
					}
				}
			}
			check(old, int(size), nil)
			check(SmallBytesRef{Slot: old.Slot}, 0, ErrInvalidSlot)
			check(SmallBytesRef{Generation: old.Generation}, 0, ErrInvalidSlot)
			smallFree(t, a, size, old)
			fresh := smallAlloc(t, a, size+1)
			if old.Slot != fresh.Slot || old.Generation == fresh.Generation {
				t.Fatalf("expected slot reuse, got %v then %v", old, fresh)
			}
			check(old, 0, ErrInvalidSlot)
			check(fresh, int(size+1), nil)
			a.Reset()
			check(fresh, 0, ErrInvalidSlot)
			newPage := smallAlloc(t, a, size)
			check(old, 0, ErrInvalidSlot)
			check(fresh, 0, ErrInvalidSlot)
			check(newPage, int(size), nil)
			a.Close()
			check(newPage, 0, ErrClosed)
			check(SmallBytesRef{}, 0, ErrClosed)
		})
	}
}

func BenchmarkSmallBytesBorrow(b *testing.B) {
	for _, size := range []uint32{32, 256} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			store, err := NewSmallBytes(SmallBytesConfig{})
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(store.Close)
			var refs [4096]SmallBytesRef
			for i := range refs {
				refs[i], err = store.Allocate(size)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data, err := store.Borrow(refs[i*593&(len(refs)-1)])
				if err != nil || len(data) != int(size) {
					b.Fatal(len(data), err)
				}
			}
		})
	}
}
