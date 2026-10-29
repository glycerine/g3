package realclosure

import "testing"

func TestTowerDenominatorPolynomialLCMScaling(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerDenominatorFixture(t, usePrem)
			normalized, denominator, err := f.base.store.cleanTowerPolynomialDenominators(
				[]*realValue{f.halfAlpha, f.thirdGamma}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			threeAlpha, err := f.base.store.multiplyRankedRetainedValues(f.base.rational(3), f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			twoGamma, err := f.base.store.multiplyRankedRetainedValues(f.two, f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(normalized, []*realValue{threeAlpha, twoGamma})
			six := f.base.rational(6)
			f.expect([]*realValue{denominator}, []*realValue{six})
			f.release(normalized, []*realValue{threeAlpha, twoGamma, denominator})
		})
	}
}
