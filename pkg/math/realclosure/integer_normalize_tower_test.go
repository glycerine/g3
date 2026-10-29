package realclosure

import "testing"

func TestTowerDenominatorIntegerContentNormalization(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerDenominatorFixture(t, usePrem)
			twoGamma, err := f.base.store.multiplyRankedRetainedValues(f.two, f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			four := f.base.rational(4)
			fourAlpha, err := f.base.store.multiplyRankedRetainedValues(four, f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			normalized, changed, err := f.base.store.normalizeTowerIntegerContent([]*realValue{twoGamma, fourAlpha}, usePrem)
			if err != nil || !changed {
				t.Fatal(changed, err)
			}
			twoAlpha, err := f.base.store.multiplyRankedRetainedValues(f.two, f.alpha, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(normalized, []*realValue{f.gamma, twoAlpha})
			f.release(normalized, []*realValue{twoGamma, fourAlpha, twoAlpha})
		})
	}
}
