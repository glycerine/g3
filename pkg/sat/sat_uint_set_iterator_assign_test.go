package sat

import (
	"reflect"
	"testing"
)

func TestSATUintSetAssignedOwnerIteratesIndependentValues(t *testing.T) {
	source, _ := satSolverSetOwner(t)
	copySet, _ := satSolverSetOwner(t)
	for _, value := range []uint32{8, 2, 5} {
		paramsMust(t, source.Insert(value))
	}
	paramsMust(t, copySet.Assign(source))
	paramsMust(t, source.Remove(2))
	it, err := copySet.Begin()
	paramsMust(t, err)
	var got []uint32
	for value, yes := it.Next(); yes; value, yes = it.Next() {
		got = append(got, value)
	}
	if !reflect.DeepEqual(got, []uint32{8, 2, 5}) {
		t.Fatal(got)
	}
}
