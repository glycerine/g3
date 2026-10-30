package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestManagerArbitraryIntegerScalarArithmetic(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	integer := numeral.NewInt(0)
	if err := f.store.numbers.Integers().SetString(&integer, "1208925819614629174706179"); err != nil {
		t.Fatal(err)
	}
	defer f.store.numbers.Integers().Del(&integer)
	one := f.rational(1)
	for _, usePrem := range []bool{false, true} {
		sum, err := f.store.addValueInteger(one, &integer, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		back, err := f.store.subtractValueInteger(sum, &integer, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		product, err := f.store.multiplyValueInteger(one, &integer, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		quotient, err := f.store.divideValueInteger(product, &integer, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		requireSameScalarResult(t, f.store, back, one, usePrem)
		requireSameScalarResult(t, f.store, quotient, one, usePrem)
		releaseScalarResults(t, f.store, sum, back, product, quotient)
	}
}
