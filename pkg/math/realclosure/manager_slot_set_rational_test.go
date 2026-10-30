package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerNumeralSlotSetRationalCopiesSource(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	source := numeral.NewRational()
	if err := f.store.numbers.SetString(&source, "7/3"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.store.numbers.Del(&source); err != nil {
			t.Error(err)
		}
	}()
	var slot *realValue
	if err := f.store.setNumeralSlotRational(&slot, &source); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.store.release(slot); err != nil {
			t.Error(err)
		}
	}()
	if err := f.store.numbers.SetInt32(&source, -9); err != nil {
		t.Fatal(err)
	}
	want := numeral.NewRational()
	if err := f.store.numbers.SetString(&want, "7/3"); err != nil {
		t.Fatal(err)
	}
	defer f.store.numbers.Del(&want)
	equal, err := f.store.equalValueRational(slot, &want)
	if err != nil || !equal {
		t.Fatal("slot did not own rational copy", equal, err)
	}
}
