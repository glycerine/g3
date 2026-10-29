package realclosure

import "testing"

func TestRankedAPIPredicates(t *testing.T) {
	f := newRankedValueFixture(t, true)
	positive, err := f.base.store.isPositiveRankedValue(f.gamma, true)
	if err != nil || !positive {
		t.Fatal(positive, err)
	}
	negativeGamma, err := f.base.store.negateRankedRetainedValue(f.gamma, true)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := f.base.store.isNegativeRankedValue(negativeGamma, true)
	if err != nil || !negative {
		t.Fatal(negative, err)
	}
	zero, err := f.base.store.isZeroRankedValue(nil, true)
	if err != nil || !zero {
		t.Fatal(zero, err)
	}
	f.releaseValues(negativeGamma)
}
