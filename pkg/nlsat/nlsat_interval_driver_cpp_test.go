package nlsat

// Source: src/test/nlsat.cpp tst3 and tst_interval.
import (
	"github.com/glycerine/g3/pkg/sat"
	"testing"
)

func TestNLSATSourceIntervalUnionCases(t *testing.T) {
	m := intervalFixture(t)
	pool := intervalPool(t, m)
	root := assignmentInteger(t, m.am, 2)
	assignmentMust(t, m.am.Root(root, 2, root))
	pool[4] = root
	negative := assignmentInteger(t, m.am, 2)
	assignmentMust(t, m.am.Root(negative, 2, negative))
	assignmentMust(t, m.am.Neg(negative))
	pool[-4] = negative
	p1, p2 := sat.MakeLiteral(1, false), sat.MakeLiteral(2, false)
	mk := func(lo int32, loOpen, loInf bool, hi int32, hiOpen, hiInf bool, why sat.Literal) *IntervalSet {
		s, e := m.Mk(loOpen, loInf, pool[lo], hiOpen, hiInf, pool[hi], why)
		assignmentMust(t, e)
		assignmentMust(t, m.IncRef(s))
		t.Cleanup(func() { assignmentMust(t, m.DecRef(s)) })
		return s
	}
	closed := func(lo, hi int32, why sat.Literal) *IntervalSet { return mk(lo, false, false, hi, false, false, why) }
	union := func(a, b *IntervalSet) *IntervalSet {
		s, e := m.MkUnion(a, b)
		assignmentMust(t, e)
		assignmentMust(t, m.IncRef(s))
		t.Cleanup(func() { assignmentMust(t, m.DecRef(s)) })
		return s
	}
	check := func(a, b *IntervalSet, want uint32) {
		t.Helper()
		r := union(a, b)
		count, e := m.NumIntervals(r)
		assignmentMust(t, e)
		if count != want {
			t.Fatalf("interval count %d want %d", count, want)
		}
		for _, s := range []*IntervalSet{a, b} {
			ok, e := m.Subset(s, r)
			assignmentMust(t, e)
			if !ok {
				t.Fatal("union did not contain input")
			}
		}
		ab, e := m.Subset(a, b)
		assignmentMust(t, e)
		ba, e := m.Subset(b, a)
		assignmentMust(t, e)
		same, e := m.SetEq(a, b)
		assignmentMust(t, e)
		if same {
			for _, s := range []*IntervalSet{a, b} {
				ok, e := m.SetEq(s, r)
				assignmentMust(t, e)
				if !ok {
					t.Fatal("equal inputs changed union")
				}
			}
		} else {
			if !ab {
				ok, e := m.Subset(r, b)
				assignmentMust(t, e)
				if ok {
					t.Fatal("union wrongly subset of second")
				}
			}
			if !ba {
				ok, e := m.Subset(r, a)
				assignmentMust(t, e)
				if ok {
					t.Fatal("union wrongly subset of first")
				}
			}
		}
		rev := union(b, a)
		equal, e := m.SetEq(r, rev)
		assignmentMust(t, e)
		if !equal {
			t.Fatal("union not commutative")
		}
		full := mk(0, true, true, 0, true, true, sat.MakeLiteral(131, false))
		equal, e = m.SetEq(r, full)
		assignmentMust(t, e)
		isFull, e := m.IsFull(r)
		assignmentMust(t, e)
		if equal != isFull {
			t.Fatal("full flag")
		}
	}
	check(closed(0, 2, p1), closed(0, 2, p2), 1)
	check(closed(0, 2, p1), closed(-4, 1, p2), 2)
	check(closed(-4, 1, p1), closed(0, 2, p2), 2)
	check(closed(-4, 1, p1), closed(2, 3, p2), 2)
	check(closed(-4, 3, p1), closed(0, 2, p2), 1)
	check(closed(-2, 2, p1), union(closed(-4, 0, p2), closed(1, 3, p2)), 2)
	check(closed(-2, 2, p1), closed(2, 3, p2), 2)
	check(closed(-2, 2, p1), mk(2, true, false, 3, false, false, p2), 2)
	check(closed(-2, 2, p1), mk(2, true, false, 3, false, false, p1), 1)
	check(mk(-2, false, false, 2, true, false, p1), mk(2, true, false, 3, false, false, p1), 2)
	check(closed(2, 2, p1), closed(2, 3, p2), 1)
	a := union(closed(-2, 0, p1), closed(1, 3, p1))
	b := mk(0, true, true, -4, false, false, p2)
	check(a, b, 3)
	b = union(b, closed(1, 4, p2))
	b = union(b, mk(2, false, false, 0, true, true, p2))
	check(a, b, 4)
	a = mk(0, true, true, 1, false, false, p1)
	b = mk(1, true, false, 0, true, true, p2)
	check(a, b, 2)
	b = union(mk(1, true, false, 2, false, false, p2), mk(2, false, false, 0, true, true, p1))
	check(a, b, 3)
}
