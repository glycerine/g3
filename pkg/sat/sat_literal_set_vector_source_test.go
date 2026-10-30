package sat

import (
	"reflect"
	"testing"
)

func TestSATLiteralSetSourceToVectorOwnsCopy(t *testing.T) {
	var set satLiteralSetStorage
	satOK(t, set.AssignLiterals([]Literal{5, 1, 9}))
	vector := set.ToVector()
	vector[0] = 13
	if !reflect.DeepEqual(set.values, []Literal{5, 1, 9}) {
		t.Fatal(set.values, vector)
	}
}
