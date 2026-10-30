package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSATLiteralSetSourcePopUsesLastValue(t *testing.T) {
	var set satLiteralSetStorage
	for _, literal := range []Literal{3, 7, 11} {
		satOK(t, set.Insert(literal))
	}
	literal, err := set.Pop()
	satOK(t, err)
	if literal != 11 || set.contains(11) || set.size() != 2 {
		t.Fatal(literal, set.values)
	}
	set.Reset()
	if _, err = set.Pop(); !errors.Is(err, arena.ErrBounds) {
		t.Fatal(err)
	}
}
