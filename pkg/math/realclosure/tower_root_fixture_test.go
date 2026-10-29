package realclosure

import "testing"

type towerRootFixture struct {
	*rankedValueFixture
	polynomial    []*realValue
	negativeAlpha *realValue
	negativeGamma *realValue
}

func newTowerRootFixture(t *testing.T, usePrem bool) *towerRootFixture {
	t.Helper()
	f := newRankedValueFixture(t, usePrem)
	negativeAlpha, err := f.base.store.negateRankedRetainedValue(f.alpha, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	negativeGamma, err := f.base.store.negateRankedRetainedValue(f.gamma, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	polynomial, err := f.base.store.multiplyRankedPolynomials(
		[]*realValue{f.alpha, f.one}, []*realValue{negativeGamma, f.one}, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &towerRootFixture{rankedValueFixture: f, polynomial: polynomial, negativeAlpha: negativeAlpha, negativeGamma: negativeGamma}
	t.Cleanup(func() { f.release(polynomial, []*realValue{negativeAlpha, negativeGamma}) })
	return fixture
}

func (f *towerRootFixture) expectSignedTowerRoots(t *testing.T, roots []*realValue, usePrem bool) {
	t.Helper()
	if len(roots) != 2 {
		t.Fatalf("roots=%d", len(roots))
	}
	foundNegative, foundPositive := false, false
	for _, root := range roots {
		sign, err := f.base.store.determineTowerRetainedValueSign(root, usePrem)
		if err != nil {
			t.Fatal(err)
		}
		if sign < 0 {
			foundNegative = true
		}
		if sign > 0 {
			foundPositive = true
		}
		if root.isRational || root.root == nil || len(root.root.polynomial.coefficients) != len(f.polynomial) {
			t.Fatal("missing tower defining polynomial")
		}
	}
	if !foundNegative || !foundPositive {
		t.Fatal("missing signed tower roots", foundNegative, foundPositive)
	}
}
