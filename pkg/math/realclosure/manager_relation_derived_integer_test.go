package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerDerivedIntegerScalarRelations(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	value := f.rational(3)
	scalar := numeral.NewInt(0)
	defer f.store.numbers.Integers().Del(&scalar)
	for _, tc := range []struct {
		n           int64
		neq, le, ge bool
	}{{3, false, true, true}, {4, true, true, false}, {2, true, false, true}} {
		if err := f.store.numbers.Integers().SetInt64(&scalar, tc.n); err != nil {
			t.Fatal(err)
		}
		neq, e0 := f.store.notEqualValueInteger(value, &scalar)
		le, e1 := f.store.lessEqualValueInteger(value, &scalar)
		ge, e2 := f.store.greaterEqualValueInteger(value, &scalar)
		if e0 != nil || e1 != nil || e2 != nil || neq != tc.neq || le != tc.le || ge != tc.ge {
			t.Fatal(tc.n, neq, le, ge, e0, e1, e2)
		}
	}
}
