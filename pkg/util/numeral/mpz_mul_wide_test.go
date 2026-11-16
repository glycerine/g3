package numeral

import (
	"math/rand"
	"testing"
)

// TestMultiplyDigitsWideMatchesSchoolbook checks the 64-bit limb product
// against the 32-bit digit schoolbook loop on random and extreme operands.
func TestMultiplyDigitsWideMatchesSchoolbook(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ref := func(a, b []uint32) []uint32 {
		c := make([]uint32, len(a)+len(b))
		for j, v := range b {
			var k uint32
			for i, u := range a {
				x := uint64(u)*uint64(v) + uint64(c[i+j]) + uint64(k)
				c[i+j] = uint32(x)
				k = uint32(x >> 32)
			}
			c[j+len(a)] = k
		}
		return c
	}
	gen := func(n int, mode int) []uint32 {
		d := make([]uint32, n)
		for i := range d {
			switch mode {
			case 0:
				d[i] = r.Uint32()
			case 1:
				d[i] = ^uint32(0)
			default:
				if r.Intn(3) == 0 {
					d[i] = r.Uint32()
				}
			}
		}
		return d
	}
	for round := 0; round < 3000; round++ {
		na, nb := 1+r.Intn(150), 1+r.Intn(150)
		a, b := gen(na, round%3), gen(nb, (round/3)%3)
		want := ref(a, b)
		got := make([]uint32, na+nb)
		for i := range got {
			got[i] = r.Uint32() // stale scratch contents
		}
		multiplyDigitsWide(a, b, got)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("na=%d nb=%d digit %d: got %x want %x", na, nb, i, got[i], want[i])
			}
		}
	}
}
