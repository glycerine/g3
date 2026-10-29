package realclosure

import "testing"

type towerSturmFixture struct {
	*rankedValueFixture
	sibling *realValue
}

func newTowerSturmFixture(t *testing.T, usePrem bool) *towerSturmFixture {
	t.Helper()
	f := newRankedValueFixture(t, usePrem)
	sibling, err := f.alpha.root.owner.rootRankedRetainedValue(f.alpha, 2, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !sibling.deleted {
			if err := f.base.store.release(sibling); err != nil {
				t.Error(err)
			}
		}
	})
	return &towerSturmFixture{rankedValueFixture: f, sibling: sibling}
}

func (f *towerSturmFixture) siblingDifferencePolynomial(t *testing.T, usePrem bool) []*realValue {
	t.Helper()
	negativeGamma, err := f.base.store.negateRankedRetainedValue(f.gamma, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	return []*realValue{negativeGamma, f.one}
}
