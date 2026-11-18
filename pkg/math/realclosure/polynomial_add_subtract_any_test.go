package realclosure

import "testing"

type anyPolynomialFixture struct {
	t     *testing.T
	base  *boundedSignFixture
	alpha *realValue
	one   *realValue
	two   *realValue
}

func newAnyPolynomialFixture(t *testing.T) *anyPolynomialFixture {
	t.Helper()
	base := newBoundedSignFixture(t, -3, 3)
	minusTwo, one, two := base.rational(-2), base.rational(1), base.rational(2)
	owner := &algebraicRootOwner{values: base.store, intervals: base.bounds}
	roots, err := owner.isolateAnyRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseIsolatedValues(t, base.store, roots) })
	return &anyPolynomialFixture{t: t, base: base, alpha: roots[1], one: one, two: two}
}

func (f *anyPolynomialFixture) expect(got, want []*realValue) {
	f.t.Helper()
	equal, err := f.base.store.structurallyEqualPolynomials(got, want)
	if err != nil || !equal {
		f.t.Fatalf("polynomial mismatch: equal=%v err=%v got=%v want=%v", equal, err, got, want)
	}
}

func (f *anyPolynomialFixture) release(p ...[]*realValue) {
	f.t.Helper()
	for _, polynomial := range p {
		if err := f.base.store.releaseSturmCoefficients(polynomial); err != nil {
			f.t.Fatal(err)
		}
	}
}
