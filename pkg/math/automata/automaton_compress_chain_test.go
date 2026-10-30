package automata

import "testing"

func TestAutomatonCompressSourceSplicesIncomingChain(t *testing.T) {
	a := New(0, []uint32{2}, []Move{
		ConditionMove(0, 1, 11),
		EpsilonMove(1, 2),
	})
	a.Compress()
	if a.MoveCount() != 1 || a.Moves()[0] != ConditionMove(0, 2, 11) {
		t.Fatalf("chain compression = %#v", a.Moves())
	}
	if !a.Accepts([]uint32{11}) {
		t.Fatal("spliced chain rejected its source word")
	}
}
