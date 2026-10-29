package realclosure

import "testing"

func TestLCMIntegerDenominatorValues(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	two, three, four := f.rational(2), f.rational(3), f.rational(4)
	lcm, err := f.store.lcmIntegerDenominators([]*realValue{two, three, four})
	if err != nil {
		t.Fatal(err)
	}
	expectRationalPolynomial(t, f, []*realValue{lcm}, 12)
	if err := f.store.release(lcm); err != nil {
		t.Fatal(err)
	}
}
