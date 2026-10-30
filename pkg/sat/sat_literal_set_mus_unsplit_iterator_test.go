package sat

import (
	"reflect"
	"testing"
)

func TestSATMUSUnsplitUsesLiteralSetIteratorOrder(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	var assignment, support satMUSLiteralSet
	for _, literal := range []Literal{0, 2, 4, 6} {
		satOK(t, assignment.insert(literal))
	}
	for _, literal := range []Literal{2, 6} {
		satOK(t, support.insert(literal))
	}
	satOK(t, s.mus.unsplit(&assignment, &support))
	if !reflect.DeepEqual(assignment.values, []Literal{0, 4}) || !reflect.DeepEqual(support.values, []Literal{2, 6}) {
		t.Fatal(assignment.values, support.values)
	}
}
