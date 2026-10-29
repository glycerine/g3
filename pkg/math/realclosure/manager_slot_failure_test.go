package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestManagerNumeralSlotForeignFailuresPreserveSlots(t *testing.T) {
	f := newRankedValueFixture(t, true)
	foreign := newRankedValueFixture(t, true)
	slot := f.alpha
	if err := f.base.store.retain(slot); err != nil {
		t.Fatal(err)
	}
	before := slot.refs
	if err := f.base.store.copyNumeralSlot(&slot, foreign.alpha); !errors.Is(err, arena.ErrInvalidSlot) || slot != f.alpha || slot.refs != before {
		t.Fatal(slot, before, slot.refs, err)
	}
	foreignSlot := foreign.alpha
	if err := f.base.store.swapNumeralSlots(&slot, &foreignSlot); !errors.Is(err, arena.ErrInvalidSlot) || slot != f.alpha || foreignSlot != foreign.alpha {
		t.Fatal(err)
	}
	if err := f.base.store.resetNumeralSlot(&slot); err != nil {
		t.Fatal(err)
	}
}
