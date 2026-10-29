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
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
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

func TestAnyPolynomialAddSubtract(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			defer f.release([]*realValue{negativeAlpha})

			sum, err := f.base.store.addAnyPolynomials(
				[]*realValue{f.alpha, f.one}, []*realValue{negativeAlpha, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(sum, []*realValue{nil, f.two})

			difference, err := f.base.store.subtractAnyPolynomials(
				[]*realValue{f.alpha, f.one}, []*realValue{f.alpha, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			if len(difference) != 0 {
				t.Fatal("expected canonical zero polynomial", difference)
			}
			f.release(sum, difference)
		})
	}
}
