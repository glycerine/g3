package realclosure

import "testing"

func TestManagerNumeralSlotSelfCopyIsNoOp(t *testing.T) {
	f := newRankedValueFixture(t, true)
	slot := f.alpha
	if err := f.base.store.retain(slot); err != nil {
		t.Fatal(err)
	}
	before := slot.refs
	if err := f.base.store.copyNumeralSlot(&slot, slot); err != nil || slot.refs != before {
		t.Fatal(before, slot.refs, err)
	}
	if err := f.base.store.resetNumeralSlot(&slot); err != nil {
		t.Fatal(err)
	}
}
