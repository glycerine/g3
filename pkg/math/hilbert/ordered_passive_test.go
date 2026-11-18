package hilbert

import "testing"

func TestOrderedPassiveAdvancesAndResetsOffsetAcrossZero(t *testing.T) {
	b := New(testQ(), nil)
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

// AUDIT.md math_misc_01-F4: passive2 orders slots with util/heap.h, whose
// erase_min moves the last value to the root, so three equal-weight inserts
// 0, 1, 2 pop as 0, 2, 1. Exhausted slots are reused from the free list.
func TestOrderedPassivePopsEqualWeightsInHeapOrder(t *testing.T) {
	b := New(testQ(), nil)
	positive := &vector{values: []int64{0, 1, 0, 0}, weight: 1}
	negatives := []*vector{
		{values: []int64{0, 1, 0, 0}, weight: -1},
		{values: []int64{0, 0, 1, 0}, weight: -1},
		{values: []int64{0, 0, 0, 1}, weight: -1},
	}
	p := newOrderedPassive(b, append([]*vector{positive}, negatives...))
	for _, n := range negatives {
		if err := p.insert(n, 0); err != nil {
			t.Fatal(err)
		}
	}
	var got []*vector
	for !p.empty() {
		sos, pas, _, err := p.pop()
		if err != nil {
			t.Fatal(err)
		}
		if sos != positive {
			t.Fatal("unexpected support")
		}
		got = append(got, pas)
	}
	want := []*vector{negatives[0], negatives[2], negatives[1]}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("pop order differs from heap.h")
	}
	if len(p.freeList) != 3 || p.freeList[0] != 0 || p.freeList[1] != 2 || p.freeList[2] != 1 {
		t.Fatalf("free list = %v, want [0 2 1]", p.freeList)
	}
	extra := &vector{values: []int64{0, 0, 1, 1}, weight: -2}
	if err := p.insert(extra, 0); err != nil {
		t.Fatal(err)
	}
	if len(p.pas) != 3 || p.pas[1] != extra {
		t.Fatal("insert must reuse the most recently freed slot")
	}
}
