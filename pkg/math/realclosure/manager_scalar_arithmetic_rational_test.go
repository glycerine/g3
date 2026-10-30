package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerRationalScalarArithmeticRoundTrips(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		f := newAnySturmFixture(t)
		scalar := numeral.NewRational()
		if err := f.base.store.numbers.SetString(&scalar, "3/2"); err != nil {
			t.Fatal(err)
		}
		defer f.base.store.numbers.Del(&scalar)
		sum, err := f.base.store.addValueRational(f.alpha, &scalar, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		back, err := f.base.store.subtractValueRational(sum, &scalar, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		product, err := f.base.store.multiplyValueRational(f.alpha, &scalar, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		quotient, err := f.base.store.divideValueRational(product, &scalar, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		requireSameScalarResult(t, f.base.store, back, f.alpha, usePrem)
		requireSameScalarResult(t, f.base.store, quotient, f.alpha, usePrem)
		releaseScalarResults(t, f.base.store, sum, back, product, quotient)
	}
}
