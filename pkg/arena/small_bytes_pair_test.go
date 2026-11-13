package arena

import (
	"errors"
	"testing"
)

func TestSmallBytesUint32Pair(t *testing.T) {
	for _, size := range []uint32{11, 512} {
		a := newSmallTest(t, 80, nil)
		r := smallAlloc(t, a, size)
		for i := uint32(0); i < size; i++ {
			if err := a.Write(r, i, byte(i*29+5)); err != nil {
				t.Fatal(err)
			}
		}
		for offset := uint32(0); offset+8 <= size; offset++ {
			w0, e0 := a.ReadUint32LE(r, offset)
			w1, e1 := a.ReadUint32LE(r, offset+4)
			g0, g1, err := a.ReadUint32Pair(r, offset)
			if e0 != nil || e1 != nil || err != nil || g0 != w0 || g1 != w1 {
				t.Fatalf("size %d offset %d: %x %x want %x %x: %v", size, offset, g0, g1, w0, w1, err)
			}
		}
		for _, offset := range []uint32{size - 7, size, ^uint32(0)} {
			if _, _, err := a.ReadUint32Pair(r, offset); !errors.Is(err, ErrBounds) {
				t.Fatal("read bounds", offset, err)
			}
			if err := a.WriteUint32Pair(r, offset, 1, 2); !errors.Is(err, ErrBounds) {
				t.Fatal("write bounds", offset, err)
			}
		}
		if err := a.WriteUint32Pair(r, 3, 0xdeadbeef, 0x01020304); err != nil {
			t.Fatal(err)
		}
		if g0, g1, err := a.ReadUint32Pair(r, 3); err != nil || g0 != 0xdeadbeef || g1 != 0x01020304 {
			t.Fatal("round trip", g0, g1, err)
		}
		if b, err := a.Read(r, 3); err != nil || b != 0xef {
			t.Fatal("little endian", b, err)
		}
		smallFree(t, a, size, r)
		replacement := smallAlloc(t, a, size)
		if _, _, err := a.ReadUint32Pair(r, 0); !errors.Is(err, ErrInvalidSlot) {
			t.Fatal("stale generation accepted", err)
		}
		if err := a.WriteUint32Pair(r, 0, 1, 2); !errors.Is(err, ErrInvalidSlot) {
			t.Fatal("stale generation write accepted", err)
		}
		smallFree(t, a, size, replacement)
	}
}
