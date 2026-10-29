package realclosure

import "testing"

func TestManagerNumeralSlotSwapTransfersWithoutRefcount(t *testing.T) {
	f := newRankedValueFixture(t, true)
	left, right := f.alpha, f.gamma
	if err := f.base.store.retain(left); err != nil {
		t.Fatal(err)
	}
	if err := f.base.store.retain(right); err != nil {
		t.Fatal(err)
	}
	leftRefs, rightRefs := left.refs, right.refs
	if err := f.base.store.swapNumeralSlots(&left, &right); err != nil || left != f.gamma || right != f.alpha || left.refs != rightRefs || right.refs != leftRefs {
		t.Fatal(left, right, err)
	}
	if err := f.base.store.resetNumeralSlot(&left); err != nil {
		t.Fatal(err)
	}
	if err := f.base.store.resetNumeralSlot(&right); err != nil {
		t.Fatal(err)
	}
}
