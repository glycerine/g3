package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerScalarAssignmentReleasesReplacedValue(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	old := f.rational(2)
	if err := f.store.retain(old); err != nil {
		t.Fatal(err)
	}
	slot := old
	input := numeral.NewRational()
	if err := f.store.numbers.SetString(&input, "5/2"); err != nil {
		t.Fatal(err)
	}
	defer f.store.numbers.Del(&input)
	if err := f.store.setNumeralSlotRational(&slot, &input); err != nil {
		t.Fatal(err)
	}
	if old.refs != 1 || old.deleted || slot == old {
		t.Fatal("replacement ownership", old.refs, old.deleted, slot == old)
	}
	if equal, err := f.store.equalValueRational(slot, &input); err != nil || !equal {
		t.Fatal(equal, err)
	}
	if err := f.store.release(slot); err != nil {
		t.Fatal(err)
	}
}
