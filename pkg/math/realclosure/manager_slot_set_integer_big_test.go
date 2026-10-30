package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerNumeralSlotSetArbitraryInteger(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	integer := numeral.NewInt(0)
	if err := f.store.numbers.Integers().SetString(&integer, "18446744073709551617001"); err != nil {
		t.Fatal(err)
	}
	defer f.store.numbers.Integers().Del(&integer)
	var slot *realValue
	if err := f.store.setNumeralSlotInteger(&slot, &integer); err != nil {
		t.Fatal(err)
	}
	if equal, err := f.store.equalValueInteger(slot, &integer); err != nil || !equal {
		t.Fatal(equal, err)
	}
	if err := f.store.numbers.Integers().SetInt64(&integer, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.store.setNumeralSlotInteger(&slot, &integer); err != nil || slot != nil {
		t.Fatal("integer zero did not reset slot", slot, err)
	}
}
