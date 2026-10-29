package realclosure

import "testing"

func TestAnyRootDispatchAppendsDistinguishedZero(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			owner := &algebraicRootOwner{values: f.base.store, intervals: f.base.bounds}
			roots, err := owner.isolateAnyRoots([]*realValue{nil, f.query[0], f.one}, usePrem)
			if err != nil || len(roots) != 2 || roots[1] != nil {
				t.Fatal("zero-root dispatch", roots, err)
			}
			equal, err := f.base.store.structurallyEqualValues(roots[0], f.alpha)
			if err != nil || !equal {
				t.Fatal("nonzero root", equal, err)
			}
			releaseIsolatedValues(t, f.base.store, roots)
		})
	}
}
