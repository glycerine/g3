package realclosure

import "testing"

func TestAnySturmSignedRemainder(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo-canonicalization"}[usePrem], func(t *testing.T) {
			f := newAnySturmFixture(t)
			derivative, err := f.base.store.derivativeAnyPolynomial(f.polynomial, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			remainder, err := f.base.store.signedRemainderAny(f.polynomial, derivative, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			want := f.expectedSignedRemainder(t, false, usePrem)
			f.expect(remainder, []*realValue{want})
			f.release(derivative, remainder, []*realValue{want})
		})
	}
}
