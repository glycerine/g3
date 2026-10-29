package realclosure

import "testing"

func TestTowerSturmSiblingEquality(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newTowerSturmFixture(t, usePrem)
			comparison, err := f.base.store.compareTowerRetainedValues(f.sibling, f.gamma, usePrem)
			if err != nil || comparison != 0 {
				t.Fatal("sibling equality", comparison, err)
			}
			reverse, err := f.base.store.compareTowerRetainedValues(f.gamma, f.sibling, usePrem)
			if err != nil || reverse != 0 {
				t.Fatal("reverse sibling equality", reverse, err)
			}
		})
	}
}
