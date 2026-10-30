package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSATLiteralSetSourceInvalidLiteralAssignmentPrefix(t *testing.T) {
	var set satLiteralSetStorage
	err := set.AssignLiterals([]Literal{2, Literal(^uint32(0)), 4})
	if !errors.Is(err, arena.ErrBounds) || set.size() != 1 || !set.contains(2) || set.contains(4) {
		t.Fatal(err, set.values)
	}
}
