package hilbert

import "testing"

func TestOrderedPassiveSkipsOppositeIntegerCoordinates(t *testing.T) {
	b := New(nil)
	b.integers[1] = true
	positive := &vector{values: []int64{0, 1}, weight: 1}
	negative := &vector{values: []int64{0, -1}, weight: -1}
	p := newOrderedPassive(b, []*vector{positive, negative})
	if err := p.insert(negative, 0); err != nil {
		t.Fatal(err)
	}
	if !p.empty() {
		t.Fatal("integer-sign-incompatible pair was queued")
	}
}
