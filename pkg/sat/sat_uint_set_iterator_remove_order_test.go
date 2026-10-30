package sat

import (
	"reflect"
	"testing"
)

func TestSATUintSetIteratorObservesSourceSwapOrder(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	for _, value := range []uint32{1, 3, 5, 7} {
		paramsMust(t, set.Insert(value))
	}
	paramsMust(t, set.Remove(3))
	it, err := set.Begin()
	paramsMust(t, err)
	var got []uint32
	for value, yes := it.Next(); yes; value, yes = it.Next() {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []uint32{1, 7, 5}) {
		t.Fatal(got)
	}
}
