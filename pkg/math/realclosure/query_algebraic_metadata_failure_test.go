package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAlgebraicMetadataRejectsForeignValue(t *testing.T) {
	f := newRankedValueFixture(t, true)
	foreign := newRankedValueFixture(t, true)
	if _, err := f.base.store.extensionIndex(foreign.alpha); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
	if _, err := f.base.store.numDefiningCoefficients(foreign.alpha); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
	if _, _, err := f.base.store.queryAlgebraicInterval(foreign.alpha); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
	if _, err := f.base.store.numSignConditions(foreign.alpha); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
}
