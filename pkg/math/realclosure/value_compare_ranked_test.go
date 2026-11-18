package realclosure

import "testing"

func TestRankedValueComparisonUsesNestedExactSign(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			comparison, err := f.base.store.compareRankedRetainedValues(f.gamma, f.alpha, usePrem)
			if err != nil || comparison != -1 {
				t.Fatal("gamma compared with alpha", comparison, err)
			}
			reverse, err := f.base.store.compareRankedRetainedValues(f.alpha, f.gamma, usePrem)
			if err != nil || reverse != 1 {
				t.Fatal("alpha compared with gamma", reverse, err)
			}
		})
	}
}

// AUDIT.md math_realclosure_02-F5: compareValues has no same-root shortcut;
// 1+gamma and 2+gamma differ and compare by sign(a - b).
func TestCompareValuesDistinctLinearAlgebraic(t *testing.T) {
	f := newRankedValueFixture(t, true)
	two := f.base.rational(2)
	x, err := f.base.store.addRankedRetainedValues(f.one, f.gamma, true)
	if err != nil {
		t.Fatal(err)
	}
	y, err := f.base.store.addRankedRetainedValues(two, f.gamma, true)
	if err != nil {
		t.Fatal(err)
	}
	defer f.releaseValues(x, y)
	comparison, err := f.base.store.compareValues(x, y, f.base.bounds)
	if err != nil || comparison != -1 {
		t.Fatal(comparison, err)
	}
	comparison, err = f.base.store.compareValues(y, x, f.base.bounds)
	if err != nil || comparison != 1 {
		t.Fatal(comparison, err)
	}
}
