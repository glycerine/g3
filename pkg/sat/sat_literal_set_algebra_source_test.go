package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetSourceIntersectionAndUnionOrder(t *testing.T) {
	var left, right satLiteralSetStorage
	satOK(t, left.AssignLiterals([]Literal{8, 2, 6, 4}))
	satOK(t, right.AssignLiterals([]Literal{4, 8, 10}))
	left.Intersect(&right)
	if !reflect.DeepEqual(left.values, []Literal{8, 4}) || left.contains(2) || left.contains(6) {
		t.Fatal(left.values)
	}
	satOK(t, left.Union(&right))
	if !reflect.DeepEqual(left.values, []Literal{8, 4, 10}) {
		t.Fatal(left.values)
	}
	left.Intersect(&left)
	satOK(t, left.Union(&left))
	if !reflect.DeepEqual(left.values, []Literal{8, 4, 10}) {
		t.Fatal(left.values)
	}
}
