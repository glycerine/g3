package rel

// Source: src/test/tbv.cpp.
import (
	"fmt"
	"testing"
)

func TestTBVSourceDriverWidthsAndIntersections(t *testing.T) {
	empty, _ := newTestTBVManager(t, 0)
	e0, err := empty.Allocate0()
	if err != nil {
		t.Fatal(err)
	}
	e1, err := empty.Allocate1()
	if err != nil {
		t.Fatal(err)
	}
	ex, err := empty.AllocateX()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []*TBV{e1, ex} {
		equal, err := empty.Equals(e0, v)
		if err != nil || !equal {
			t.Fatal(equal, err)
		}
	}
	for _, width := range []uint32{31, 11, 15, 16, 17} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m, _ := newTestTBVManager(t, width)
			z, err := m.Allocate0()
			if err != nil {
				t.Fatal(err)
			}
			o, err := m.Allocate1()
			if err != nil {
				t.Fatal(err)
			}
			x, err := m.AllocateX()
			if err != nil {
				t.Fatal(err)
			}
			n, err := m.AllocateUint64(31)
			if err != nil {
				t.Fatal(err)
			}
			eq := func(a, b *TBV, want bool) {
				t.Helper()
				got, err := m.Equals(a, b)
				if err != nil || got != want {
					t.Fatal(got, want, err)
				}
			}
			eq(o, z, false)
			eq(o, x, false)
			eq(z, x, false)
			_, err = m.SetAnd(x, z)
			if err != nil {
				t.Fatal(err)
			}
			eq(z, x, true)
			eq(o, x, false)
			if err = m.Copy(x, o); err != nil {
				t.Fatal(err)
			}
			eq(o, x, true)
			eq(z, x, false)
			if err = m.Fill(x, TBitX); err != nil {
				t.Fatal(err)
			}
			ok, err := m.Intersect(x, z, n)
			if err != nil || !ok {
				t.Fatal(ok, err)
			}
			eq(z, n, true)
			ok, err = m.Intersect(z, o, n)
			if err != nil || ok {
				t.Fatal(ok, err)
			}
			if err = m.Fill(o, TBit1); err != nil {
				t.Fatal(err)
			}
			if err = m.Set(o, 2, TBit0); err != nil {
				t.Fatal(err)
			}
			if err = m.Set(o, 4, TBitX); err != nil {
				t.Fatal(err)
			}
			deleted := make([]bool, width)
			deleted[1] = true
			deleted[3] = true
			destination, _ := newTestTBVManager(t, width-2)
			projected, err := destination.Project(deleted, o)
			if err != nil {
				t.Fatal(err)
			}
			for dst, src := uint32(0), uint32(0); src < width; src++ {
				if deleted[src] {
					continue
				}
				a, e := m.Get(o, src)
				if e != nil {
					t.Fatal(e)
				}
				b, e := destination.Get(projected, dst)
				if e != nil || a != b {
					t.Fatal(src, dst, a, b, e)
				}
				dst++
			}
			for i := uint64(0); i < 55; i++ {
				v, err := m.AllocateUint64(i)
				if err != nil {
					t.Fatal(err)
				}
				w, err := m.AllocateUint64(i + 1)
				if err != nil {
					t.Fatal(err)
				}
				valid, err := m.WellFormed(v)
				if err != nil || !valid {
					t.Fatal(i, valid, err)
				}
				valid, err = m.SetAnd(w, v)
				if err != nil || valid {
					t.Fatal(i, valid, err)
				}
				valid, err = m.WellFormed(w)
				if err != nil || valid {
					t.Fatal(i, valid, err)
				}
				if err = m.Deallocate(v); err != nil {
					t.Fatal(err)
				}
				if err = m.Deallocate(w); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
