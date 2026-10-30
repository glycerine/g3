package automata

import "testing"

func TestAutomatonCompressSourceTrimsAndSinkifiesDeadStates(t *testing.T) {
	a := New(0, []uint32{2}, []Move{ConditionMove(0, 1, 3)})
	a.Compress()
	if !a.IsEmpty() || a.NumStates() != 2 || a.MoveCount() != 0 {
		t.Fatalf("dead-state compression: states=%d finals=%v moves=%v", a.NumStates(), a.FinalStates(), a.Moves())
	}
	// sinkify_dead_states clears outgoing edges only; the source inverse table
	// retains its incoming record.
	if a.InDegree(1) != 1 {
		t.Fatalf("source inverse-edge retention lost: %v", a.MovesTo(1))
	}
}
