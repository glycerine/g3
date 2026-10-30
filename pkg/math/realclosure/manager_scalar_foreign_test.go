package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestManagerScalarAssignmentRejectsMissingNumerals(t *testing.T) {
	local := newBoundedSignFixture(t, -1, 1)
	var slot *realValue
	if err := local.store.setNumeralSlotRational(&slot, nil); !errors.Is(err, arena.ErrInvalidSlot) || slot != nil {
		t.Fatal("nil rational", slot, err)
	}
	if err := local.store.setNumeralSlotInteger(&slot, nil); !errors.Is(err, arena.ErrInvalidSlot) || slot != nil {
		t.Fatal("nil integer", slot, err)
	}
	if _, err := local.store.compareValueRational(nil, nil); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal("nil rational comparison", err)
	}
}
