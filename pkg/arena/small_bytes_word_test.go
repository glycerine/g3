package arena

import (
	"errors"
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
