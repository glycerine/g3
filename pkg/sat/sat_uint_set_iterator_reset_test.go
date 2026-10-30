package sat

import "testing"

func TestSATUintSetIteratorAfterResetAndFinalizeIsEmpty(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	paramsMust(t, set.Insert(4))
	paramsMust(t, set.Reset())
	it, err := set.Begin()
	paramsMust(t, err)
	if _, yes := it.Next(); yes {
		t.Fatal("reset set iterated an old value")
	}
	paramsMust(t, set.Insert(6))
	paramsMust(t, set.Finalize())
	it, err = set.Begin()
	paramsMust(t, err)
	if _, yes := it.Next(); yes {
		t.Fatal("finalized set iterated an old value")
	}
}
