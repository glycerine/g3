package sat

import (
	"reflect"
	"testing"
)

func TestSATMUSSplitUsesLiteralSetIteratorOrder(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	var left, right satMUSLiteralSet
	for _, literal := range []Literal{0, 2, 4, 6, 8} {
		satOK(t, left.insert(literal))
	}
	satOK(t, s.mus.split(&left, &right))
	if !reflect.DeepEqual(left.values, []Literal{0, 2}) || !reflect.DeepEqual(right.values, []Literal{4, 6, 8}) {
		t.Fatal(left.values, right.values)
	}
}
