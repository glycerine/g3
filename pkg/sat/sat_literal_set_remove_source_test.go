package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetSourceRemoveSwapsLast(t *testing.T) {
	var set satLiteralSetStorage
	for _, literal := range []Literal{2, 4, 6, 8} {
		satOK(t, set.Insert(literal))
	}
	satOK(t, set.Remove(4))
	if set.contains(4) || !reflect.DeepEqual(set.values, []Literal{2, 8, 6}) {
		t.Fatal(set.values, set.membership)
	}
	satOK(t, set.Remove(10))
	if !reflect.DeepEqual(set.values, []Literal{2, 8, 6}) {
		t.Fatal(set.values)
	}
}
