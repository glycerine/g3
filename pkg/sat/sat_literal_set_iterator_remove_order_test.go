package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetIteratorObservesSourceSwapOrder(t *testing.T) {
	var set satLiteralSetStorage
	satOK(t, set.AssignLiterals([]Literal{1, 3, 5, 7}))
	satOK(t, set.Remove(3))
	it := set.Begin()
	var got []Literal
	for literal, yes := it.Next(); yes; literal, yes = it.Next() {
		got = append(got, literal)
	}
	if !reflect.DeepEqual(got, []Literal{1, 7, 5}) {
		t.Fatal(got)
	}
}
