package bv

import "testing"

func TestBVSliceSourceLiftsNestedExtractBoundary(t *testing.T) {
	f := newBVSliceFixture(t)
	frame := newBVSliceFrame(f.tactic)
	defer frame.Close()
	nested := f.extract(t, 15, 0, f.extract(t, 23, 8, f.x))
	sharingMust(t, frame.register(4, 11, nested))
	points := frame.points(f.x)
	if len(points) != 2 || points[0] != 12 || points[1] != 20 {
		t.Fatal(points)
	}
}
