package rel

import (
	"errors"
	"testing"
)

func TestSparseFunctionalConsumersRejectMismatchedMetadata(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 8}, 1)
	ordinary := newSparseFunctionalFixture(t, []uint64{4, 8}, 0)
	if _, err := target.UnionFunctional(ordinary, nil); !errors.Is(err, ErrTable) {
		t.Fatalf("union metadata=%v", err)
	}
	if _, err := target.SelectEqualProjectFunctional(0, 2); !errors.Is(err, ErrTable) {
		t.Fatalf("select column=%v", err)
	}
	if _, err := target.ComplementFunctional(nil); !errors.Is(err, ErrTable) {
		t.Fatalf("complement suffix=%v", err)
	}
	wide := newSparseFunctionalFixture(t, []uint64{2, 2, 8}, 1)
	if _, err := wide.ComplementFunctional(Fact{0}); !errors.Is(err, ErrTable) {
		t.Fatalf("wide complement=%v", err)
	}
}
