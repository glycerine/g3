package rel

// Source: src/test/doc.cpp tst_doc1.
import (
	"fmt"
	"math/big"
	"testing"
)

func TestDOCSourceDriverCubeOperations(t *testing.T) {
	for _, width := range []uint32{5, 10, 70} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m, _ := newTestDocManager(t, width)
			must := func(e error) {
				t.Helper()
				if e != nil {
					t.Fatal(e)
				}
			}
			alloc := func(d *Doc, e error) *Doc { t.Helper(); must(e); return d }
			eq := func(a, b *Doc, want bool) {
				t.Helper()
				same, e := m.Equal(a, b)
				must(e)
				if same != want {
					t.Fatal("document equality", same, want)
				}
			}
			d := alloc(m.Allocate())
			d1 := alloc(m.Allocate1())
			d0 := alloc(m.Allocate0())
			dx := alloc(m.AllocateX())
			dxc := alloc(m.AllocateCopy(dx))
			d10 := alloc(m.AllocateUint64(10))
			_ = alloc(m.AllocateBigInt(big.NewInt(20)))
			if width < 64 {
				_ = alloc(m.AllocateUint64Range(0xff, 3, 1))
			}
			must(m.Copy(d, d10))
			eq(d, d10, true)
			must(m.Fill0(d))
			eq(d, d10, false)
			must(m.Fill0(d10))
			eq(d, d10, true)
			must(m.Fill1(d))
			must(m.Deallocate(d10))
			d10 = alloc(m.AllocateUint64(10))
			eq(d, d10, false)
			eq(d, d1, true)
			must(m.FillX(d))
			eq(d, dx, true)
			full, e := m.IsFull(dx)
			must(e)
			if !full {
				t.Fatal("X not full")
			}
			full, e = m.IsFull(d1)
			must(e)
			if full {
				t.Fatal("one full")
			}
			ok, e := m.SetAnd(dx, dx)
			must(e)
			if !ok {
				t.Fatal("self intersection")
			}
			eq(dxc, dx, true)
			ok, e = m.SetAnd(dx, d1)
			must(e)
			if !ok {
				t.Fatal("one intersection")
			}
			eq(dxc, dx, false)
			eq(dx, d1, true)
			ok, e = m.FoldNeg(dx)
			must(e)
			if !ok {
				t.Fatal("fold")
			}
			must(m.Deallocate(dx))
			dx = alloc(m.AllocateX())
			for _, tc := range []struct {
				a, b *Doc
				want bool
			}{{dx, d1, true}, {dx, d0, true}, {d0, d1, false}, {d1, d0, false}} {
				got, e := m.Contains(tc.a, tc.b)
				must(e)
				if got != tc.want {
					t.Fatal("contains", got, tc.want)
				}
			}
			negative, e := m.vectors.Allocate0()
			must(e)
			must(m.AddNegativeOwned(d1, negative))
			ok, e = m.FoldNeg(d1)
			must(e)
			if !ok {
				t.Fatal("fold negation")
			}
			deleted := make([]bool, width)
			deleted[1] = true
			deleted[3] = true
			dst, _ := newTestDocManager(t, width-2)
			projected := allocDocProjection(t, m, dst, deleted, d1)
			expected, e := dst.Allocate1()
			must(e)
			same, e := dst.Equal(projected, expected)
			must(e)
			if !same {
				t.Fatal("projection of ones")
			}
			must(m.Set(d1, 2, TBitX))
			must(m.Set(d1, 4, TBitX))
			_ = allocDocProjection(t, m, dst, deleted, d1)
			negative, e = m.vectors.Allocate1()
			must(e)
			must(m.AddNegativeOwned(d1, negative))
			well, e := m.WellFormed(d1)
			must(e)
			if !well {
				t.Fatal("source document malformed")
			}
			_ = allocDocProjection(t, m, dst, deleted, d1)
		})
	}
}

func allocDocProjection(t *testing.T, source, destination *DocManager, deleted []bool, doc *Doc) *Doc {
	t.Helper()
	result, e := source.Project(destination, deleted, doc)
	if e != nil {
		t.Fatal(e)
	}
	return result
}
