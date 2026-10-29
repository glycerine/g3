package realclosure

import "testing"

func TestManagerNumeralSlotSetInteger(t *testing.T) {
	f := newRankedValueFixture(t, true)
	var slot *realValue
	if err := f.base.store.setNumeralSlotInt64(&slot, 7); err != nil {
		t.Fatal(err)
	}
	equal, err := f.base.store.structurallyEqualValues(slot, f.base.rational(7))
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	if err := f.base.store.setNumeralSlotInt64(&slot, 0); err != nil || slot != nil {
		t.Fatal(slot, err)
	}
}
