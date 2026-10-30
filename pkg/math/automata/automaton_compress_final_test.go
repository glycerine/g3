package automata

import "testing"

func TestAutomatonCompressSourcePreservesFinalEpsilonSource(t *testing.T) {
	a := New(0, []uint32{1, 3}, []Move{
		ConditionMove(0, 1, 4),
		EpsilonMove(1, 2),
		ConditionMove(2, 3, 5),
	})
	a.Compress()
	if !a.Accepts([]uint32{4}) || !a.Accepts([]uint32{4, 5}) {
		t.Fatalf("final-source splice changed language: %#v", a.Moves())
	}
	if a.Accepts([]uint32{5}) {
		t.Fatal("final-source splice introduced a word")
	}
}
