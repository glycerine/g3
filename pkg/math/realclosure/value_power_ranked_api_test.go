package realclosure

import "testing"

func TestRankedAPIValuePower(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			square, err := f.base.store.powerRankedRetainedValue(f.gamma, 2, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			equal, err := f.base.store.structurallyEqualValues(square, f.alpha)
			if err != nil || !equal {
				t.Fatal("gamma squared", equal, err)
			}
			fourth, err := f.base.store.powerRankedRetainedValue(f.gamma, 4, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect([]*realValue{fourth}, []*realValue{f.two})
			zero, err := f.base.store.powerRankedRetainedValue(f.gamma, 0, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			f.expect([]*realValue{zero}, []*realValue{f.one})
			f.releaseValues(square, fourth, zero)
		})
	}
}
