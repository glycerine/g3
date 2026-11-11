package automata

import (
	"bytes"
	"testing"
)

func TestAutomatonSequenceUnionConcatOptionalAndReverse(t *testing.T) {
	one := Sequence([]uint32{1})
	two := Sequence([]uint32{2})
	concat := Concat(one, two)
	if !concat.Accepts([]uint32{1, 2}) || concat.Accepts([]uint32{1}) || concat.Accepts([]uint32{2, 1}) {
		t.Fatal("concat language")
	}
	union := Union(one, two)
	if !union.Accepts([]uint32{1}) || !union.Accepts([]uint32{2}) || union.Accepts([]uint32{1, 2}) {
		t.Fatal("union language")
	}
	optional := Optional(concat)
	if !optional.Accepts(nil) || !optional.Accepts([]uint32{1, 2}) || optional.Accepts([]uint32{1}) {
		t.Fatal("optional language")
	}
	reverse := Reverse(concat)
	if !reverse.Accepts([]uint32{2, 1}) || reverse.Accepts([]uint32{1, 2}) {
		t.Fatal("reverse language")
	}
}

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
	a.AddFinalToInitialMoves()
	if !a.Accepts([]uint32{7, 7}) {
		t.Fatal("final-to-initial loop")
	}
	a.Compress()
	// z3 4.7.1's compress keeps the final-to-initial epsilon move of a
	// final source (0 -7-> 3, 3 -e-> 0), so only the language is checked.
	if !a.Accepts([]uint32{7}) || !a.Accepts([]uint32{7, 7}) || a.Accepts(nil) {
		t.Fatal("compressed language", a.Moves())
	}
	sequence := Sequence([]uint32{4, 5, 6})
	if length, ok := sequence.IsSequence(); !ok || length != 3 {
		t.Fatal("sequence", length, ok)
	}
}

func TestAutomatonEmptyEpsilonCloneSinkAndDisplay(t *testing.T) {
	if !Empty().IsEmpty() || !Epsilon().IsEpsilon() || !Epsilon().Accepts(nil) {
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
