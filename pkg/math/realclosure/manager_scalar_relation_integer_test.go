package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerIntegerScalarRelations(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	value := f.rational(-4)
	scalar := numeral.NewInt(0)
	defer f.store.numbers.Integers().Del(&scalar)
	for _, tc := range []struct {
		value                int64
		equal, less, greater bool
	}{{-4, true, false, false}, {-3, false, true, false}, {-5, false, false, true}} {
		if err := f.store.numbers.Integers().SetInt64(&scalar, tc.value); err != nil {
			t.Fatal(err)
		}
		equal, e0 := f.store.equalValueInteger(value, &scalar)
		less, e1 := f.store.lessValueInteger(value, &scalar)
		greater, e2 := f.store.greaterValueInteger(value, &scalar)
		if e0 != nil || e1 != nil || e2 != nil || equal != tc.equal || less != tc.less || greater != tc.greater {
			t.Fatal(tc.value, equal, less, greater, e0, e1, e2)
		}
	}
}
