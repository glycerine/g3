package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetLiveMUSAndConsequenceConsumers(t *testing.T) {
	var left, right satMUSLiteralSet
	for _, literal := range []Literal{0, 2, 4, 6} {
		satOK(t, left.insert(literal))
	}
	for _, literal := range []Literal{2, 6, 8} {
		satOK(t, right.insert(literal))
	}
	left.intersect(&right)
	satOK(t, left.union(&right))
	if !reflect.DeepEqual(left.values, []Literal{2, 6, 8}) {
		t.Fatal(left.values)
	}
	satOK(t, satConsequenceSetRemove(&left, 6))
	if !reflect.DeepEqual(left.values, []Literal{2, 8}) || left.contains(6) {
		t.Fatal(left.values)
	}
	var copy satMUSLiteralSet
	copy.assign(&left)
	satOK(t, left.insert(10))
	if !reflect.DeepEqual(copy.values, []Literal{2, 8}) {
		t.Fatal(copy.values)
	}
}
