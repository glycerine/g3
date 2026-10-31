package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
)

func TestIntervalVectorJoinProject(t *testing.T) {
	rangeOf := func(lo, hi int64) smtcore.RelationInterval {
		return smtcore.NewRelationInterval(big.NewRat(lo, 1), false, big.NewRat(hi, 1), false)
	}
	left, right := newIntervalColumns(2, false), newIntervalColumns(2, false)
	astTestMust(t, left.intersect(0, rangeOf(0, 4)))
	astTestMust(t, left.equate(0, 1))
	astTestMust(t, right.intersect(0, rangeOf(2, 6)))
	joined, err := joinIntervalColumns(left, right, []uint32{0, 1}, []uint32{0, 1})
	astTestMust(t, err)
	if joined.empty || joined.find(0) != joined.find(2) || joined.find(1) != joined.find(3) {
		t.Fatal("join did not equate selected columns")
	}
	if joined.find(0) == joined.find(1) {
		t.Fatal("source join does not copy input equivalences")
	}
	bound := joined.columns[joined.find(0)]
	if !bound.Contains(big.NewRat(2, 1)) || !bound.Contains(big.NewRat(4, 1)) || bound.Contains(big.NewRat(1, 1)) || bound.Contains(big.NewRat(5, 1)) {
		t.Fatal("join failed to intersect bounds")
	}
	projected, err := projectIntervalColumns(joined, []uint32{0, 3})
	astTestMust(t, err)
	if projected.find(0) == projected.find(1) {
		t.Fatal("projection merged different classes")
	}
	projected, err = projectIntervalColumns(joined, []uint32{0, 1})
	astTestMust(t, err)
	if !projected.columns[projected.find(0)].Contains(big.NewRat(3, 1)) {
		t.Fatal("projection lost bounds")
	}
	astTestMust(t, left.equate(0, 1))
	projected, err = projectIntervalColumns(left, nil)
	astTestMust(t, err)
	if projected.find(0) != projected.find(1) {
		t.Fatal("projection lost equivalence")
	}
	if _, err = projectIntervalColumns(left, []uint32{1, 0}); err == nil {
		t.Fatal("unsorted removed columns accepted")
	}
	if _, err = joinIntervalColumns(left, right, []uint32{2}, []uint32{0}); err == nil {
		t.Fatal("invalid join column accepted")
	}
	empty, err := joinIntervalColumns(newIntervalColumns(1, true), right, nil, nil)
	astTestMust(t, err)
	if !empty.empty {
		t.Fatal("join of empty relation must be empty")
	}
}

func TestIntervalVectorUnionAndWiden(t *testing.T) {
	point := func(v int64) smtcore.RelationInterval {
		n := big.NewRat(v, 1)
		return smtcore.NewRelationInterval(n, false, n, false)
	}
	target, source := newIntervalColumns(2, false), newIntervalColumns(2, false)
	astTestMust(t, target.intersect(0, point(1)))
	astTestMust(t, target.intersect(1, point(1)))
	astTestMust(t, target.equate(0, 1))
	astTestMust(t, source.intersect(0, point(1)))
	astTestMust(t, source.intersect(1, point(2)))
	changed, err := unionIntervalColumns(target, source, false)
	astTestMust(t, err)
	if !changed || target.find(0) == target.find(1) {
		t.Fatal("union must split unequal equivalence classes")
	}
	if !target.columns[target.find(1)].Contains(big.NewRat(1, 1)) || !target.columns[target.find(1)].Contains(big.NewRat(2, 1)) {
		t.Fatal("union must include both bounds")
	}
	changed, err = unionIntervalColumns(target, source, false)
	astTestMust(t, err)
	if changed {
		t.Fatal("repeating union should not report a change")
	}

	widenTarget, widenSource := newIntervalColumns(1, false), newIntervalColumns(1, false)
	astTestMust(t, widenTarget.intersect(0, point(1)))
	astTestMust(t, widenSource.intersect(0, point(2)))
	changed, err = unionIntervalColumns(widenTarget, widenSource, true)
	astTestMust(t, err)
	_, _, hi, _ := widenTarget.columns[0].Bounds()
	if !changed || hi != nil {
		t.Fatal("widening should extend upper bound to infinity")
	}

	empty := newIntervalColumns(2, true)
	changed, err = unionIntervalColumns(empty, target, false)
	astTestMust(t, err)
	if !changed || empty.empty || !empty.columns[1].Contains(big.NewRat(2, 1)) {
		t.Fatal("union into empty relation failed")
	}
	changed, err = unionIntervalColumns(target, newIntervalColumns(2, true), false)
	astTestMust(t, err)
	if changed {
		t.Fatal("empty source should not change union")
	}
}
