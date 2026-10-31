package rel

// Source: src/test/doc.cpp tst_doc1.
import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/util/backtrack"
	z3rand "github.com/glycerine/g3/pkg/util/random"
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

// Source: src/test/doc.cpp test_doc_cls::test_project1 and test_project.
func TestDOCSourceRandomProjection(t *testing.T) {
	source, _ := newTestDocManager(t, 4)
	destinations := make([]*DocManager, 5)
	for i := uint32(1); i <= 4; i++ {
		destinations[i], _ = newTestDocManager(t, i)
	}
	rng := z3rand.New(0)
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	check := func(d *Doc, deleted []bool) {
		t.Helper()
		kept := uint32(0)
		for _, remove := range deleted {
			if !remove {
				kept++
			}
		}
		target := destinations[kept]
		p, e := source.Project(target, deleted, d)
		must(e)
		for value := uint32(0); value < 1<<kept; value++ {
			want := false
			for removed := uint32(0); removed < 1<<(4-kept); removed++ {
				if independentDocMatches(t, source, d, projectedAssignment(value, deleted, removed)) {
					want = true
					break
				}
			}
			if got := independentDocMatches(t, target, p, value); got != want {
				t.Fatalf("projection mask %v value %d got %v want %v", deleted, value, got, want)
			}
		}
		must(target.Deallocate(p))
	}
	first, e := source.AllocateX()
	must(e)
	neg, e := source.vectors.AllocateX()
	must(e)
	must(source.vectors.Set(neg, 0, TBit0))
	must(source.AddNegativeOwned(first, neg))
	check(first, []bool{true, false, false, false})
	must(source.Deallocate(first))
	choose := func() TBit {
		switch rng.Bounded(3) {
		case 0:
			return TBit0
		case 1:
			return TBit1
		default:
			return TBitX
		}
	}
	for trial := 0; trial < 200; trial++ {
		pos, e := source.vectors.Allocate()
		must(e)
		for i := uint32(0); i < 4; i++ {
			must(source.vectors.Set(pos, i, choose()))
		}
		d, e := source.AllocateTBVCopy(pos)
		must(e)
		must(source.vectors.Deallocate(pos))
		for j := 0; j < 3; j++ {
			v, e := source.vectors.Allocate()
			must(e)
			for i := uint32(0); i < 4; i++ {
				bit, e := source.vectors.Get(d.positive, i)
				must(e)
				if bit == TBitX {
					bit = choose()
				}
				must(source.vectors.Set(v, i, bit))
			}
			must(source.AddNegativeOwned(d, v))
		}
		valid, e := source.WellFormed(d)
		must(e)
		if !valid {
			t.Fatal("generated document invalid", trial)
		}
		deleted := make([]bool, 4)
		for i := 1; i < 4; i++ {
			deleted[i] = rng.Bounded(2) == 0
		}
		check(d, deleted)
		must(source.Deallocate(d))
	}
}

// Source: src/test/doc.cpp test_intersect and the randomized portion of test_subtract.
func TestDOCSourceRandomUnionOperations(t *testing.T) {
	m, _ := newTestDocManager(t, 4)
	rng := z3rand.New(0)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	choose := func() TBit {
		switch rng.Bounded(3) {
		case 0:
			return TBit0
		case 1:
			return TBit1
		default:
			return TBitX
		}
	}
	// Advance through the preceding 200 projection rounds in the same driver.
	for i := 0; i < 200; i++ {
		var pos [4]TBit
		for j := range pos {
			pos[j] = choose()
		}
		for j := 0; j < 3; j++ {
			for _, v := range pos {
				if v == TBitX {
					_ = choose()
				}
			}
		}
		for j := 0; j < 3; j++ {
			_ = rng.Bounded(2)
		}
	}
	randomDoc := func() *Doc {
		pos, e := m.vectors.Allocate()
		must(e)
		for i := uint32(0); i < 4; i++ {
			must(m.vectors.Set(pos, i, choose()))
		}
		d, e := m.AllocateTBVCopy(pos)
		must(e)
		must(m.vectors.Deallocate(pos))
		for j := 0; j < 3; j++ {
			neg, e := m.vectors.Allocate()
			must(e)
			for i := uint32(0); i < 4; i++ {
				bit, e := m.vectors.Get(d.positive, i)
				must(e)
				if bit == TBitX {
					bit = choose()
				}
				must(m.vectors.Set(neg, i, bit))
			}
			must(m.AddNegativeOwned(d, neg))
		}
		valid, e := m.WellFormed(d)
		must(e)
		if !valid {
			t.Fatal("source generated malformed document")
		}
		return d
	}
	randomUnion := func() *DocUnion {
		u, e := NewDocUnion(m)
		must(e)
		for i := 0; i < 3; i++ {
			must(u.PushOwned(randomDoc()))
		}
		return u
	}
	for _, tc := range []struct {
		name     string
		rounds   int
		subtract bool
	}{{"intersection", 10000, false}, {"subtraction", 1000, true}} {
		for round := 0; round < tc.rounds; round++ {
			left, right := randomUnion(), randomUnion()
			var expected [16]bool
			for a := uint32(0); a < 16; a++ {
				l, r := independentUnionMatches(t, left, a), independentUnionMatches(t, right, a)
				if tc.subtract {
					expected[a] = l && !r
				} else {
					expected[a] = l && r
				}
			}
			if tc.subtract {
				must(left.Subtract(right))
			} else {
				must(left.Intersect(right))
				valid, e := left.WellFormed()
				must(e)
				if !valid {
					t.Fatal("intersection malformed", round)
				}
			}
			for a := uint32(0); a < 16; a++ {
				if got := independentUnionMatches(t, left, a); got != expected[a] {
					t.Fatalf("%s round %d assignment %04b got %v want %v", tc.name, round, a, got, expected[a])
				}
			}
			must(left.Reset())
			must(right.Reset())
		}
	}
	// The source runs 200 merge rounds after the union operations.
	for round := 0; round < 200; round++ {
		d, e := m.AllocateX()
		must(e)
		for i := 0; i < 7; i++ {
			neg, e := m.vectors.Allocate()
			must(e)
			for bit := uint32(0); bit < 4; bit++ {
				must(m.vectors.Set(neg, bit, choose()))
			}
			must(m.AddNegativeOwned(d, neg))
		}
		classes := backtrack.NewDefaultUnionFind()
		for i := 0; i < 4; i++ {
			classes.MkVar()
		}
		var selected [4]bool
		anchor := uint32(4)
		for i := uint32(1); i < 4; i++ {
			selected[i] = rng.Bounded(2) == 0
			if selected[i] {
				anchor = i
			}
		}
		if anchor == 4 {
			must(m.Deallocate(d))
			continue
		}
		for i := uint32(0); i < 4; i++ {
			if selected[i] && i != anchor {
				classes.Merge(i, anchor)
			}
		}
		var expected [16]bool
		for a := uint32(0); a < 16; a++ {
			expected[a] = independentDocMatches(t, m, d, a)
			for i := uint32(0); i < 4; i++ {
				if selected[i] && (a>>i)&1 != (a>>anchor)&1 {
					expected[a] = false
				}
			}
		}
		valid, e := m.MergeRange(d, anchor, 1, classes, make([]bool, 4))
		must(e)
		for a := uint32(0); a < 16; a++ {
			got := valid && independentDocMatches(t, m, d, a)
			if got != expected[a] {
				t.Fatalf("merge round %d assignment %04b got %v want %v", round, a, got, expected[a])
			}
		}
		must(m.Deallocate(d))
	}
}

// Source: src/test/doc.cpp test_subtract's concrete example.
func TestDOCSourceConcreteSubtraction(t *testing.T) {
	m, _ := newTestDocManager(t, 4)
	left, e := NewDocUnion(m)
	if e != nil {
		t.Fatal(e)
	}
	right, e := NewDocUnion(m)
	if e != nil {
		t.Fatal(e)
	}
	a, e := m.AllocateX()
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Set(a, 0, TBit0); e != nil {
		t.Fatal(e)
	}
	if e = m.Set(a, 1, TBit1); e != nil {
		t.Fatal(e)
	}
	if e = left.PushOwned(a); e != nil {
		t.Fatal(e)
	}
	b, e := m.AllocateX()
	if e != nil {
		t.Fatal(e)
	}
	for _, bits := range [][2]TBit{{TBit1, TBit0}, {TBit0, TBit1}} {
		v, e := m.vectors.AllocateX()
		if e != nil {
			t.Fatal(e)
		}
		if e = m.vectors.Set(v, 0, bits[0]); e != nil {
			t.Fatal(e)
		}
		if e = m.vectors.Set(v, 2, bits[1]); e != nil {
			t.Fatal(e)
		}
		if e = m.AddNegativeOwned(b, v); e != nil {
			t.Fatal(e)
		}
	}
	if e = right.PushOwned(b); e != nil {
		t.Fatal(e)
	}
	var expected [16]bool
	for a := uint32(0); a < 16; a++ {
		expected[a] = independentUnionMatches(t, left, a) && !independentUnionMatches(t, right, a)
	}
	if e = left.Subtract(right); e != nil {
		t.Fatal(e)
	}
	for a := uint32(0); a < 16; a++ {
		if got := independentUnionMatches(t, left, a); got != expected[a] {
			t.Fatalf("assignment %04b got %v want %v", a, got, expected[a])
		}
	}
	if e = left.Reset(); e != nil {
		t.Fatal(e)
	}
	if e = right.Reset(); e != nil {
		t.Fatal(e)
	}
}
