package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestIntervalRelationRangeOperations(t *testing.T) {
	q, err := numeral.NewRationalManager(numeral.RationalConfig{})
	if err != nil {
		t.Fatal(err)
	}
	m := NewRelationIntervalManager(q)
	var owned []*RelationInterval
	keep := func(i RelationInterval, err error) *RelationInterval {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		owned = append(owned, &i)
		return &i
	}
	defer func() {
		for _, i := range owned {
			if err := m.Del(i); err != nil {
				t.Error(err)
			}
		}
	}()
	r := func(n int32) *numeral.Rational { v := smtQInt(n); return &v }
	contains := func(i *RelationInterval, n int32) bool {
		t.Helper()
		ok, err := i.Contains(r(n))
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	meet := func(a, b *RelationInterval) (*RelationInterval, bool) {
		t.Helper()
		i, empty, err := m.Meet(a, b)
		return keep(i, err), empty
	}
	a := keep(m.Bounds(r(0), true, r(2), false))
	b := keep(m.Bounds(r(0), false, r(2), true))
	h := keep(m.Unite(a, b))
	if !contains(h, 0) || !contains(h, 2) {
		t.Fatal("union endpoint openness")
	}
	i, empty := meet(a, b)
	if empty || contains(i, 0) || contains(i, 2) || !contains(i, 1) {
		t.Fatal("intersection endpoint openness")
	}
	w := keep(m.Widen(a, b))
	if !w.MinusInfinity() || !w.PlusInfinity() {
		t.Fatal("source widening must expand both endpoints")
	}
	reverse := keep(m.Widen(b, a))
	if reverse.MinusInfinity() || reverse.PlusInfinity() {
		t.Fatal("source widening is asymmetric")
	}
	if _, empty = meet(a, keep(m.Bounds(r(2), false, r(3), false))); empty {
		t.Fatal("closed endpoint is shared")
	}
	if _, empty = meet(b, keep(m.Bounds(r(2), false, r(3), false))); !empty {
		t.Fatal("open endpoint is disjoint")
	}
	if got := keep(m.HalfBound(r(-1), true, false)).String(); got != "(-oo, -1)" {
		t.Fatalf("half bound display %q", got)
	}
	if got := keep(m.Point(r(3))).String(); got != "[3, 3]" {
		t.Fatalf("point display %q", got)
	}
	half := numeral.NewRational()
	if err := q.SetString(&half, "-1/2"); err != nil {
		t.Fatal(err)
	}
	if got := keep(m.HalfBound(&half, false, true)).String(); got != "[-1/2, oo)" {
		t.Fatalf("fraction display %q", got)
	}
}
