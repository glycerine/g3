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
