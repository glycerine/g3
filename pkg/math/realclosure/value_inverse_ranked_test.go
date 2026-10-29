package realclosure

import "testing"

func TestRankedFieldValueInverse(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			inverse, err := f.base.store.inverseRankedRetainedValue(f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			product, err := f.base.store.multiplyRankedRetainedValues(f.gamma, inverse, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := f.base.store.structurallyEqualValues(product, f.one)
			if err != nil || !equal {
				t.Fatal("gamma inverse product", equal, err)
			}
			f.releaseValues(inverse, product)
		})
	}
}
