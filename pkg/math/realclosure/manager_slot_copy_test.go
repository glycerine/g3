package realclosure

import "testing"

func TestManagerNumeralSlotCopyOwnership(t *testing.T) {
	f := newRankedValueFixture(t, true)
	var slot *realValue
	before := f.gamma.refs
	if err := f.base.store.copyNumeralSlot(&slot, f.gamma); err != nil || slot != f.gamma || f.gamma.refs != before+1 {
		t.Fatal(slot, before, f.gamma.refs, err)
	}
	if err := f.base.store.resetNumeralSlot(&slot); err != nil || slot != nil || f.gamma.refs != before {
		t.Fatal(slot, before, f.gamma.refs, err)
	}
}
