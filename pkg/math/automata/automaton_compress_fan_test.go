package automata

import "testing"

func TestAutomatonCompressSourceAllEpsilonIncomingFan(t *testing.T) {
	a := New(0, []uint32{3}, []Move{
		EpsilonMove(0, 2),
		EpsilonMove(1, 2),
		ConditionMove(2, 3, 9),
	})
	a.Compress()
	want0, want1 := ConditionMove(0, 3, 9), ConditionMove(1, 3, 9)
	found0, found1 := false, false
	for _, move := range a.Moves() {
		found0 = found0 || move == want0
		found1 = found1 || move == want1
		if move.Epsilon || move.Source == 2 {
			t.Fatalf("unspliced fan move %#v in %#v", move, a.Moves())
		}
	}
	if !found0 || !found1 || !a.Accepts([]uint32{9}) {
		t.Fatalf("fan compression = %#v", a.Moves())
	}
}
