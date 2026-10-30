package sat

import "testing"

func TestSATLiteralSetSourceEmptyIterator(t *testing.T) {
	var set satLiteralSetStorage
	it := set.Begin()
	if literal, yes := it.Next(); yes || literal != 0 {
		t.Fatal(literal, yes)
	}
}
