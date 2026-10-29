package realclosure

import "testing"

func TestTowerDenominatorCleanQueryAndNoOpAPI(t *testing.T) {
	f := newTowerDenominatorFixture(t, true)
	clean, err := f.base.store.hasCleanTowerDenominator(f.halfAlpha)
	if err != nil || clean {
		t.Fatal(clean, err)
	}
	numerator, denominator, err := f.base.store.cleanTowerValueIfNeeded(f.gamma, true)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := f.base.store.structurallyEqualValues(numerator, f.gamma)
	if err != nil || !equal {
		t.Fatal(equal, err)
	}
	f.expect([]*realValue{denominator}, []*realValue{f.one})
	clean, err = f.base.store.hasCleanTowerDenominator(numerator)
	if err != nil || !clean {
		t.Fatal(clean, err)
	}
	f.releaseValues(numerator, denominator)
}
