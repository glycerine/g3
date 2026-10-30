package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetSourceIteratorInsertionOrder(t *testing.T) {
	var set satLiteralSetStorage
	satOK(t, set.AssignLiterals([]Literal{9, 3, 9, 7}))
	it := set.Begin()
	var got []Literal
	for {
		literal, yes := it.Next()
		if !yes {
			break
		}
		got = append(got, literal)
	}
	if !reflect.DeepEqual(got, []Literal{9, 3, 7}) {
		t.Fatal(got)
	}
	if _, yes := it.Next(); yes {
		t.Fatal("ended iterator resumed")
	}
}
