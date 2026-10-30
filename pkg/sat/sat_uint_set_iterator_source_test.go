package sat

import (
	"reflect"
	"testing"
)

func TestSATUintSetSourceIteratorInsertionOrder(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	for _, value := range []uint32{9, 3, 9, 7} {
		paramsMust(t, set.Insert(value))
	}
	it, err := set.Begin()
	paramsMust(t, err)
	var got []uint32
	for value, yes := it.Next(); yes; value, yes = it.Next() {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []uint32{9, 3, 7}) {
		t.Fatal(got)
	}
	if _, yes := it.Next(); yes {
		t.Fatal("ended iterator resumed")
	}
}
