package hilbert

import "testing"

func TestOrderedPassiveAdvancesAndResetsOffsetAcrossZero(t *testing.T) {
	b := New(nil)
	positive := &vector{values: []int64{0, 1}, weight: 2}
	negative := &vector{values: []int64{0, 1}, weight: -3}
	p := newOrderedPassive(b, []*vector{positive, negative})
	if err := p.insert(negative, 0); err != nil {
		t.Fatal(err)
	}
	sos, pas, offset, err := p.pop()
	if err != nil {
		t.Fatal(err)
	}
	if sos != positive || pas != negative || offset != 0 {
		t.Fatalf("pop = %p,%p,%d; want %p,%p,0", sos, pas, offset, positive, negative)
	}
}
