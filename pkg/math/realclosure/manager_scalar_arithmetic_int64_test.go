package realclosure

import "testing"

func TestManagerIntScalarArithmeticRoundTrips(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		f := newAnySturmFixture(t)
		sum, err := f.base.store.addValueInt64(f.alpha, -7, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		back, err := f.base.store.subtractValueInt64(sum, -7, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		product, err := f.base.store.multiplyValueInt64(f.alpha, -7, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		quotient, err := f.base.store.divideValueInt64(product, -7, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		requireSameScalarResult(t, f.base.store, back, f.alpha, usePrem)
		requireSameScalarResult(t, f.base.store, quotient, f.alpha, usePrem)
		releaseScalarResults(t, f.base.store, sum, back, product, quotient)
	}
}
