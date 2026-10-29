package realclosure

import "testing"

type anySturmFixture struct {
	*anyPolynomialFixture
	polynomial []*realValue
	query      []*realValue
	owned      []*realValue
}

// polynomial is (x-alpha)(x-1), query is x-alpha, alpha=sqrt(2).
func newAnySturmFixture(t *testing.T) *anySturmFixture {
	t.Helper()
	f := newAnyPolynomialFixture(t)
	negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
	if err != nil {
		t.Fatal(err)
	}
	alphaPlusOne, err := f.base.store.addRetainedValues(f.alpha, f.one)
	if err != nil {
		t.Fatal(err)
	}
	negativeSum, err := f.base.store.negateRetainedValue(alphaPlusOne)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &anySturmFixture{
		anyPolynomialFixture: f,
		polynomial:           []*realValue{f.alpha, negativeSum, f.one},
		query:                []*realValue{negativeAlpha, f.one},
		owned:                []*realValue{negativeAlpha, alphaPlusOne, negativeSum},
	}
	t.Cleanup(func() { f.release(fixture.owned) })
	return fixture
}

func (f *anySturmFixture) expectedSignedRemainder(t *testing.T, scaled bool, usePrem bool) *realValue {
	t.Helper()
	twoAlpha, err := f.base.store.multiplyAnyCanonicalValues(f.two, f.alpha, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	three := f.base.rational(3)
	numerator, err := f.base.store.subtractAnyCanonicalValues(three, twoAlpha, usePrem)
	if err != nil {
		t.Fatal(err)
	}
	f.release([]*realValue{twoAlpha})
	if scaled {
		return numerator
	}
	four := f.base.rational(4)
	result, err := f.base.store.divideRetainedValues(numerator, four)
	f.release([]*realValue{numerator})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
