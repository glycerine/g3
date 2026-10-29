package realclosure

import "testing"

type towerDenominatorFixture struct {
	*rankedValueFixture
	half       *realValue
	third      *realValue
	halfAlpha  *realValue
	thirdGamma *realValue
}

func newTowerDenominatorFixture(t *testing.T, usePrem bool) *towerDenominatorFixture {
	t.Helper()
	f := newRankedValueFixture(t, usePrem)
	half, err := f.base.store.newRational(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	third, err := f.base.store.newRational(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	halfAlpha, err := f.base.store.multiplyRankedRetainedValues(f.alpha, half, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	thirdGamma, err := f.base.store.multiplyRankedRetainedValues(f.gamma, third, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &towerDenominatorFixture{rankedValueFixture: f, half: half, third: third, halfAlpha: halfAlpha, thirdGamma: thirdGamma}
	t.Cleanup(func() { f.releaseValues(half, third, halfAlpha, thirdGamma) })
	return fixture
}
