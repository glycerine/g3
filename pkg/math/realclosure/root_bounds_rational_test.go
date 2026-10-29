package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestKnuthRationalRootBoundsAndIntervals(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	minusTwo, one := f.rational(-2), f.rational(1)
	p := []*realValue{minusTwo, nil, one}
	posUpper, ok, err := f.store.positiveRootUpperBound(p, f.bounds)
	if err != nil || !ok {
		t.Fatal(posUpper, ok, err)
	}
	posLower, ok, err := f.store.positiveRootLowerBound(p, f.bounds)
	if err != nil || !ok {
		t.Fatal(posLower, ok, err)
	}
	negLower, ok, err := f.store.negativeRootLowerBound(p, f.bounds)
	if err != nil || !ok {
		t.Fatal(negLower, ok, err)
	}
	negUpper, ok, err := f.store.negativeRootUpperBound(p, f.bounds)
	if err != nil || !ok {
		t.Fatal(negUpper, ok, err)
	}
	if posLower >= posUpper || negLower != posUpper || negUpper != posLower {
		t.Fatal("unexpected symmetric magnitudes", negLower, negUpper, posLower, posUpper)
	}
	positive, err := makeRootBoundInterval(f.bounds, posLower, true, posUpper, true, false)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := makeRootBoundInterval(f.bounds, negLower, true, negUpper, true, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := positive.Close(); err != nil {
			t.Error(err)
		}
		if err := negative.Close(); err != nil {
			t.Error(err)
		}
	})
	zero := numeral.NewBinaryRational()
	if err := f.bounds.SetInt32(&zero, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.bounds.Del(&zero) })
	if positive.lowerInf || positive.upperInf || negative.lowerInf || negative.upperInf {
		t.Fatal("rational bounds unexpectedly infinite")
	}
	positiveLower, _ := f.bounds.Less(&zero, &positive.lower)
	negativeUpper, _ := f.bounds.Less(&negative.upper, &zero)
	if !positiveLower || !negativeUpper {
		t.Fatal("bound signs")
	}
}

func TestRootBoundHelpersBinomialAndPowers(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	one, two := f.rational(1), f.rational(2)
	if !isNonzeroBinomial([]*realValue{one, nil, two}) || isNonzeroBinomial([]*realValue{one, two, two}) {
		t.Fatal("binomial classifier")
	}
	for _, test := range []struct {
		magnitude int32
		negative  bool
	}{{-3, false}, {0, false}, {7, false}, {-3, true}, {7, true}} {
		value := numeral.NewBinaryRational()
		if err := magnitudeToBinary(f.bounds, test.magnitude, test.negative, &value); err != nil {
			t.Fatal(err)
		}
		negative, err := f.bounds.IsNeg(&value)
		if err != nil || negative != test.negative {
			t.Fatalf("magnitude %d negative=%v sign=%v err=%v", test.magnitude, test.negative, negative, err)
		}
		if err := f.bounds.Del(&value); err != nil {
			t.Fatal(err)
		}
	}
}
