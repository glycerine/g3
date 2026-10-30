package api

import "testing"

func TestAPIOptimizeMinimizeBoundOrientation(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Assert(f.upper))
	index, err := f.opt.Minimize(f.x)
	astTestMust(t, err)
	_, err = f.opt.Check()
	astTestMust(t, err)
	lower, err := f.opt.Lower(index)
	astTestMust(t, err)
	upper, err := f.opt.Upper(index)
	astTestMust(t, err)
	if a, b := optimizeBoundNumeral(t, f, &lower), optimizeBoundNumeral(t, f, &upper); a != "0" || b != "0" {
		t.Fatal("bounds", a, b)
	}
	astTestMust(t, f.m.DelRef(&lower))
	astTestMust(t, f.m.DelRef(&upper))
}
