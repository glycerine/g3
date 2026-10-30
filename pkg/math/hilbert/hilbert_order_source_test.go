package hilbert

import "testing"

func TestSourceBasisIsSortedByCoordinateMagnitude(t *testing.T) {
	b := New(nil)
	if err := b.AddEQ(rationals(1, 1, -2), rational(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Saturate(); err != nil {
		t.Fatal(err)
	}
	var previous uint64
	for index, value := range b.basis {
		magnitude, err := vectorMagnitude(value)
		if err != nil {
			t.Fatal(err)
		}
		if index != 0 && magnitude < previous {
			t.Fatalf("basis magnitude %d follows %d", magnitude, previous)
		}
		previous = magnitude
	}
}
