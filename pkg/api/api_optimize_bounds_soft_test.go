package api

import (
	"math/big"
	"testing"
)

func TestAPIOptimizeSoftBoundsUseOptimalCost(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-bound-soft-p", boolean))
	index, err := f.opt.AssertSoft(p, big.NewRat(3, 1), "bound-soft")
	astTestMust(t, err)
	astTestMust(t, f.opt.Assert(f.pin(f.c.Not(p))))
	_, err = f.opt.Check()
	astTestMust(t, err)
	lower, err := f.opt.Lower(index)
	astTestMust(t, err)
	upper, err := f.opt.Upper(index)
	astTestMust(t, err)
	if a, b := optimizeBoundNumeral(t, f, &lower), optimizeBoundNumeral(t, f, &upper); a != "3" || b != "3" {
		t.Fatal("soft bounds", a, b)
	}
	astTestMust(t, f.m.DelRef(&lower))
	astTestMust(t, f.m.DelRef(&upper))
}
