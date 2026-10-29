package realclosure

import "testing"

func TestAnyPolynomialLongDivision(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newAnyPolynomialFixture(t)
			negativeAlpha, err := f.base.store.negateRetainedValue(f.alpha)
			if err != nil {
				t.Fatal(err)
			}
			minusTwo := f.base.rational(-2)
			quotient, remainder, err := f.base.store.divideAnyPolynomials(
				[]*realValue{minusTwo, nil, f.one}, []*realValue{negativeAlpha, f.one}, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect(quotient, []*realValue{f.alpha, f.one})
			if len(remainder) != 0 {
				t.Fatal("nonzero exact-division remainder", remainder)
			}
			f.release(quotient, remainder, []*realValue{negativeAlpha})
		})
	}
}
