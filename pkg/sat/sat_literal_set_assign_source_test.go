package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetSourceAssignmentsOwnValues(t *testing.T) {
	var source, copy satLiteralSetStorage
	satOK(t, source.AssignLiterals([]Literal{0, 4, 0, 8}))
	copy.Assign(&source)
	satOK(t, source.Remove(4))
	if !reflect.DeepEqual(copy.values, []Literal{0, 4, 8}) || !copy.contains(4) {
		t.Fatal(copy.values)
	}
	copy.Assign(&copy)
	if !reflect.DeepEqual(copy.values, []Literal{0, 4, 8}) {
		t.Fatal(copy.values)
	}
}
