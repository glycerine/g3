package bv

import (
	"testing"
)

func TestBVBoundsModularIntersectionAndImplication(t *testing.T) {
	a := bvBoundsRangeSet(8, 4, 20)
	b := bvBoundsRangeSet(8, 10, 30)
	got := a.intersect(b)
	want := bvBoundsRangeSet(8, 10, 20)
	if !got.equal(want) || !got.tight {
		t.Fatalf("intersection = %#v", got)
	}
	if !got.implies(a) || !got.implies(b) || a.implies(got) {
		t.Fatal("source interval implication relation was not retained")
	}

	loose := bvBoundsLooseRangeSet(8, 12, 18)
	got = a.intersect(loose)
	if got.tight || len(got.ranges) != 1 || int64(got.ranges[0].lo) != 12 || int64(got.ranges[0].hi) != 18 {
		t.Fatalf("loose intersection = %#v", got)
	}
}
