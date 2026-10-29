package realclosure

import "testing"

func TestRefineRationalCoefficientIntervals(t *testing.T) {
	f := newBoundedSignFixture(t, -1, 1)
	third := fixtureFraction(t, f, 1, 3)
	if err := f.store.refineRationalCoefficientIntervals([]*realValue{third}, f.interval, 14); err != nil {
		t.Fatal(err)
	}
	precise, err := binaryIntervalPrecise(third.rationalInterval, 14)
	if err != nil || !precise {
		t.Fatal(precise, err)
	}
}
