package realclosure

import "testing"

func TestTowerDenominatorCleanNestedValue(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerDenominatorFixture(t, usePrem)
			numerator, denominator, err := f.base.store.cleanTowerValueDenominator(f.thirdGamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := f.base.store.structurallyEqualValues(numerator, f.gamma)
			if err != nil || !equal {
				t.Fatal("clean numerator", equal, err)
			}
			three := f.base.rational(3)
			f.expect([]*realValue{denominator}, []*realValue{three})
			f.releaseValues(numerator, denominator)
		})
	}
}
