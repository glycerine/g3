package api

import (
	"math/big"
	"testing"
)

func TestAPIOptimizationObjectivesPreserveMixedInsertionOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	x := f.arithmeticVariable("objective-order-x", true)
	boolean := f.pin(f.m.MkBoolSort())
	name, err := f.m.Symbols.Intern("objective-order-p")
	astTestMust(t, err)
	p := f.pin(f.m.MkConst(name, boolean))
	astTestMust(t, f.c.Minimize(x))
	astTestMust(t, f.c.AssertSoft(p, big.NewRat(5, 1), "soft-middle"))
	astTestMust(t, f.c.Maximize(x))
	count, err := f.c.OptimizationObjectiveCount()
	astTestMust(t, err)
	objectives, err := f.c.OptimizationObjectives()
	astTestMust(t, err)
	if count != 3 || len(objectives) != 3 {
		t.Fatal("mixed objectives", count, len(objectives))
	}
	first, err := f.m.RefGet(objectives[0])
	astTestMust(t, err)
	if first != x {
		t.Fatal("first objective", first)
	}
	middle, err := f.m.RefGet(objectives[1])
	astTestMust(t, err)
	if kind, kindErr := f.m.GetDeclKind(middle); kindErr != nil || kind != 6 {
		t.Fatal("middle objective", kind, kindErr)
	}
	last, err := f.m.RefGet(objectives[2])
	astTestMust(t, err)
	if kind, kindErr := f.m.GetDeclKind(last); kindErr != nil || kind != 8 {
		t.Fatal("last objective", kind, kindErr)
	}
	for i := range objectives {
		astTestMust(t, f.m.DelRef(&objectives[i]))
	}
}
