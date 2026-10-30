package sat

import "testing"

func TestSATUintSetSourceEmptyIterator(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	it, err := set.Begin()
	paramsMust(t, err)
	if value, yes := it.Next(); yes || value != 0 {
		t.Fatal(value, yes)
	}
}
