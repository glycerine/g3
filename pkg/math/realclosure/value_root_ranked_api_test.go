package realclosure

import "testing"

func TestRankedAPIRootReconstructsNestedValue(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			root, err := f.alpha.root.owner.rootRankedRetainedValue(f.alpha, 2, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			positive, err := f.base.store.isPositiveRankedValue(root, usePrem)
			if err != nil || !positive {
				t.Fatal("independently isolated positive square root", positive, err)
			}
			squared, err := f.base.store.powerRankedRetainedValue(root, 2, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := f.base.store.structurallyEqualValues(squared, f.alpha)
			if err != nil || !equal {
				t.Fatal("new root square", equal, err)
			}
			f.releaseValues(root, squared)
		})
	}
}
