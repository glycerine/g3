package nlsat

// Source: src/test/nlsat.cpp tst3 and tst_interval.
import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
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

// Source: src/test/nlsat.cpp mk_random, tst4 and check_subset_result.
func TestNLSATSourceRandomIntervalJustifications(t *testing.T) {
	m := intervalFixture(t)
	gen := z3rand.New(0)
	lower := assignmentInteger(t, m.am, 0)
	upper := assignmentInteger(t, m.am, 0)
	set := func(out *polynomial.Algebraic, n int32) {
		v := unumeral.NewInt(n)
		assignmentMust(t, m.am.SetInteger(out, &v))
	}
	makeSet := func(lo int32, loOpen, loInf bool, hi int32, hiOpen, hiInf bool, why sat.Literal) *IntervalSet {
		set(lower, lo)
		set(upper, hi)
		s, e := m.Mk(loOpen, loInf, lower, hiOpen, hiInf, upper, why)
		assignmentMust(t, e)
		assignmentMust(t, m.IncRef(s))
		return s
	}
	merge := func(a, b *IntervalSet) *IntervalSet {
		s, e := m.MkUnion(a, b)
		assignmentMust(t, e)
		assignmentMust(t, m.IncRef(s))
		return s
	}
	randomSet := func(span, space, tries int32, why sat.Literal) *IntervalSet {
		var result *IntervalSet
		prev := -span + int32(gen.Bounded(uint32(space*4)))
		if gen.Bounded(3) == 0 {
			next := prev + int32(gen.Bounded(uint32(space)))
			open := gen.Bounded(2) == 0
			result = makeSet(0, true, true, next, open, false, why)
			prev = next
		}
		for i := int32(0); i < tries; i++ {
			lo := prev + int32(gen.Bounded(uint32(space)))
			hi := lo + int32(gen.Bounded(uint32(space)))
			loOpen := gen.Bounded(2) == 0
			hiOpen := gen.Bounded(2) == 0
			if (loOpen || hiOpen) && lo == hi {
				hi++
			}
			cur := makeSet(lo, loOpen, false, hi, hiOpen, false, why)
			next := merge(result, cur)
			assignmentMust(t, m.DecRef(result))
			assignmentMust(t, m.DecRef(cur))
			result = next
			prev = hi
		}
		if gen.Bounded(3) == 0 {
			next := prev + int32(gen.Bounded(uint32(space)))
			open := gen.Bounded(2) == 0
			cur := makeSet(next, open, false, 0, true, true, why)
			u := merge(result, cur)
			assignmentMust(t, m.DecRef(result))
			assignmentMust(t, m.DecRef(cur))
			result = u
		}
		return result
	}
	p1, p2 := sat.MakeLiteral(1, false), sat.MakeLiteral(2, false)
	full := makeSet(0, true, true, 0, true, true, sat.MakeLiteral(131, false))
	defer func() { assignmentMust(t, m.DecRef(full)) }()
	for _, config := range []struct{ span, space, tries int32 }{{20, 3, 10}, {200, 100, 20}} {
		for i := 0; i < 100; i++ {
			a, b := randomSet(config.span, config.space, config.tries, p1), randomSet(config.span, config.space, config.tries, p2)
			result := merge(a, b)
			var justifications []sat.Literal
			assignmentMust(t, m.GetJustifications(result, &justifications))
			if len(justifications) > 2 {
				t.Fatal("too many justifications", justifications)
			}
			count, e := m.NumIntervals(result)
			assignmentMust(t, e)
			for j := uint32(0); j < count; j++ {
				item, e := m.GetInterval(result, j)
				assignmentMust(t, e)
				assignmentMust(t, m.IncRef(item))
				assignmentMust(t, m.GetJustifications(item, &justifications))
				if len(justifications) != 1 {
					t.Fatal("interval justification", justifications)
				}
				source := a
				if justifications[0] != p1 {
					if justifications[0] != p2 {
						t.Fatal("unexpected justification", justifications)
					}
					source = b
				}
				ok, e := m.Subset(item, source)
				assignmentMust(t, e)
				if !ok {
					t.Fatal("reason interval exceeds its source", config, i, j)
				}
				assignmentMust(t, m.DecRef(item))
			}
			for _, input := range []*IntervalSet{a, b} {
				ok, e := m.Subset(input, result)
				assignmentMust(t, e)
				if !ok {
					t.Fatal("union lost an input", config, i)
				}
			}
			ab, e := m.Subset(a, b)
			assignmentMust(t, e)
			ba, e := m.Subset(b, a)
			assignmentMust(t, e)
			if !ab {
				ok, e := m.Subset(result, b)
				assignmentMust(t, e)
				if ok {
					t.Fatal("union unexpectedly fits second input", config, i)
				}
			}
			if !ba {
				ok, e := m.Subset(result, a)
				assignmentMust(t, e)
				if ok {
					t.Fatal("union unexpectedly fits first input", config, i)
				}
			}
			isFull, e := m.IsFull(result)
			assignmentMust(t, e)
			equalFull, e := m.SetEq(result, full)
			assignmentMust(t, e)
			if isFull != equalFull {
				t.Fatal("full flag", config, i)
			}
			reversed := merge(b, a)
			same, e := m.SetEq(result, reversed)
			assignmentMust(t, e)
			if !same {
				t.Fatal("union not commutative", config, i)
			}
			assignmentMust(t, m.DecRef(reversed))
			assignmentMust(t, m.DecRef(result))
			assignmentMust(t, m.DecRef(b))
			assignmentMust(t, m.DecRef(a))
		}
	}
}
