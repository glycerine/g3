package realclosure

import "testing"

type rankedValueFixture struct {
	*anySturmFixture
	gammaRoots []*realValue
	gamma      *realValue
}

// gamma is the positive fourth root of two: gamma^2 = alpha, alpha^2 = 2.
func newRankedValueFixture(t *testing.T, usePrem bool) *rankedValueFixture {
	t.Helper()
	f := newAnySturmFixture(t)
	owner := f.alpha.root.owner
	roots, err := owner.isolateAnyRoots([]*realValue{f.query[0], nil, f.one}, usePrem)
	if err != nil || len(roots) != 2 {
		t.Fatal("nested roots", len(roots), err)
	}
	positive := roots[1]
	sign, err := f.base.store.determineRankedRetainedValueSign(positive, usePrem)
	if err != nil || sign != 1 {
		t.Fatal("positive nested root", sign, err)
	}
	t.Cleanup(func() { releaseIsolatedValues(t, f.base.store, roots) })
	return &rankedValueFixture{anySturmFixture: f, gammaRoots: roots, gamma: positive}
}

func (f *rankedValueFixture) releaseValues(values ...*realValue) {
	f.t.Helper()
	for _, value := range values {
		if err := f.base.store.release(value); err != nil {
			f.t.Fatal(err)
		}
	}
}
