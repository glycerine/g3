package sat

import "testing"

func TestSATLiteralSetIteratorAfterResetAndFinalizeIsEmpty(t *testing.T) {
	var set satLiteralSetStorage
	satOK(t, set.AssignLiterals([]Literal{1, 5}))
	set.Reset()
	it := set.Begin()
	if _, yes := it.Next(); yes {
		t.Fatal("reset set iterated an old value")
	}
	satOK(t, set.AssignLiterals([]Literal{3}))
	set.Finalize()
	it = set.Begin()
	if _, yes := it.Next(); yes {
		t.Fatal("finalized set iterated an old value")
	}
}
