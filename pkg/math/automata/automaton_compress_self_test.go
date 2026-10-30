package automata

import "testing"

func TestAutomatonCompressSourceRemovesSelfEpsilon(t *testing.T) {
	a := New(0, []uint32{1}, []Move{
		EpsilonMove(0, 0),
		ConditionMove(0, 1, 7),
	})
	a.Compress()
	if a.MoveCount() != 1 || a.Moves()[0] != ConditionMove(0, 1, 7) {
		t.Fatalf("self epsilon compression = %#v", a.Moves())
	}
	if !a.Accepts([]uint32{7}) || a.Accepts(nil) {
		t.Fatal("self epsilon compression changed the language")
	}
}
