package realclosure

import "testing"

func TestAnySturmSignedPseudoRemainder(t *testing.T) {
	f := newAnySturmFixture(t)
	derivative, err := f.base.store.derivativeAnyPolynomial(f.polynomial, true)
	if err != nil {
		t.Fatal(err)
	}
	remainder, err := f.base.store.signedPseudoRemainderAny(f.polynomial, derivative, true)
	if err != nil {
		t.Fatal(err)
	}
	want := f.expectedSignedRemainder(t, true, true)
	f.expect(remainder, []*realValue{want})
	f.release(derivative, remainder, []*realValue{want})
}
