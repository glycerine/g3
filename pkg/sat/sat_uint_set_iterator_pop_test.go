package sat

import (
	"reflect"
	"testing"
)

func TestSATUintSetIteratorAfterPopUsesRemainingPrefix(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	for _, value := range []uint32{2, 4, 6} {
		paramsMust(t, set.Insert(value))
	}
	value, err := set.Pop()
	paramsMust(t, err)
	if value != 6 {
		t.Fatal(value)
	}
	it, err := set.Begin()
	paramsMust(t, err)
	var got []uint32
	for value, yes := it.Next(); yes; value, yes = it.Next() {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []uint32{2, 4}) {
		t.Fatal(got)
	}
}
