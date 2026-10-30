package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerRationalScalarRelations(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	value := f.rational(3)
	scalar := numeral.NewRational()
	defer f.store.numbers.Del(&scalar)
	for _, tc := range []struct {
		text                 string
		equal, less, greater bool
	}{{"3", true, false, false}, {"7/2", false, true, false}, {"5/2", false, false, true}} {
		if err := f.store.numbers.SetString(&scalar, tc.text); err != nil {
			t.Fatal(err)
		}
		equal, e0 := f.store.equalValueRational(value, &scalar)
		less, e1 := f.store.lessValueRational(value, &scalar)
		greater, e2 := f.store.greaterValueRational(value, &scalar)
		if e0 != nil || e1 != nil || e2 != nil || equal != tc.equal || less != tc.less || greater != tc.greater {
			t.Fatal(tc.text, equal, less, greater, e0, e1, e2)
		}
	}
}
