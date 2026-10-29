package realclosure

import "testing"

func TestRankedValueMultiplicationAndAlgebraicReduction(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			product, err := f.base.store.multiplyRankedRetainedValues(f.gamma, f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			if product == nil || product.root != f.gamma.root {
				t.Fatal("cross-rank product was not represented in higher extension")
			}
			f.expect(product.functionNumerator.coefficients, []*realValue{nil, f.alpha})

			square, err := f.base.store.multiplyRankedRetainedValues(f.gamma, f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := f.base.store.structurallyEqualValues(square, f.alpha)
			if err != nil || !equal {
				t.Fatal("gamma square did not reduce to alpha", equal, err)
			}
			f.releaseValues(product, square)
		})
	}
}
