package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerDerivedRationalScalarRelations(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	value := f.rational(3)
	scalar := numeral.NewRational()
	defer f.store.numbers.Del(&scalar)
	for _, tc := range []struct {
		text        string
		neq, le, ge bool
	}{{"3", false, true, true}, {"7/2", true, true, false}, {"5/2", true, false, true}} {
		if err := f.store.numbers.SetString(&scalar, tc.text); err != nil {
			t.Fatal(err)
		}
		neq, e0 := f.store.notEqualValueRational(value, &scalar)
		le, e1 := f.store.lessEqualValueRational(value, &scalar)
		ge, e2 := f.store.greaterEqualValueRational(value, &scalar)
		if e0 != nil || e1 != nil || e2 != nil || neq != tc.neq || le != tc.le || ge != tc.ge {
			t.Fatal(tc.text, neq, le, ge, e0, e1, e2)
		}
	}
}
