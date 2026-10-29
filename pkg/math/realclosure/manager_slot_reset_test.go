package realclosure

import "testing"

func TestManagerNumeralSlotResetDeletesLastOwner(t *testing.T) {
	f := newRankedValueFixture(t, true)
	slot, err := f.base.store.newRational(19, 1)
	if err != nil {
		t.Fatal(err)
	}
	value := slot
	if err := f.base.store.resetNumeralSlot(&slot); err != nil || slot != nil || !value.deleted || value.refs != 0 {
		t.Fatal(slot, value.deleted, value.refs, err)
	}
}
