package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestAccumulateAlgebraicIntegerContent(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one, six := f.rational(-2), f.rational(1), f.rational(6)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	sixAlpha, err := f.store.multiplyRetainedValues(roots[1], six)
	if err != nil {
		t.Fatal(err)
	}
	gcd := numeral.NewInt(0)
	clean, err := f.store.accumulateIntegerContent(sixAlpha, &gcd)
	if err != nil || !clean {
		t.Fatal(clean, err)
	}
	got, err := f.store.numbers.Integers().Uint64(&gcd)
	if err != nil || got != 6 {
		t.Fatal(got, err)
	}
	if err := f.store.releaseSturmInteger(&gcd); err != nil {
		t.Fatal(err)
	}
	if err := f.store.release(sixAlpha); err != nil {
		t.Fatal(err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
