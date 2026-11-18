package automata

import (
	"bytes"
	"testing"
)

func TestAutomatonEpsilonClosureFinalLoopsAndSequence(t *testing.T) {
	a := New(0, []uint32{3}, []Move{
		EpsilonMove(0, 1),
		ConditionMove(1, 2, 7),
		EpsilonMove(2, 3),
	})
	closure := a.EpsilonClosure(0)
	if len(closure) != 2 || closure[0] != 0 || closure[1] != 1 {
		t.Fatal("epsilon closure", closure)
	}
	if !a.Accepts([]uint32{7}) || a.IsEpsilonFree() {
		t.Fatal("epsilon acceptance")
	}
	a.Compress()
	if !a.Accepts([]uint32{7}) || a.Accepts(nil) {
		t.Fatal("compressed language", a.Moves())
	}
	sequence := Sequence([]uint32{4, 5, 6})
	if length, ok := sequence.IsSequence(); !ok || length != 3 {
		t.Fatal("sequence", length, ok)
	}
}

func TestAutomatonEmptyEpsilonCloneSinkAndDisplay(t *testing.T) {
	if !Empty().IsEmpty() || !Epsilon().Accepts(nil) {
		t.Fatal("empty/epsilon constructors")
	}
	loop := Loop(9)
	if !loop.Accepts(nil) || !loop.Accepts([]uint32{9, 9}) || !loop.IsLoopState(0) {
		t.Fatal("loop language")
	}
	a := New(0, []uint32{1}, []Move{ConditionMove(0, 1, 3), ConditionMove(2, 2, 4)})
	if !a.IsSink(2) || a.IsSink(1) {
		t.Fatal("sink classification")
	}
	clone := a.Clone()
	clone.RemoveFinal(1)
	if !clone.IsEmpty() || a.IsEmpty() {
		t.Fatal("clone independence")
	}
	var out bytes.Buffer
	if err := a.Display(&out); err != nil || out.String() != "init: 0\nfinal: 1 \n0 -> 1 if 3\n2 -> 2 if 4\n" {
		t.Fatal("display", out.String(), err)
	}
}
