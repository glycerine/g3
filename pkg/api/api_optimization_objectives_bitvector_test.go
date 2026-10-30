package api

import "testing"

func TestAPIOptimizationObjectivesNormalizeBitVectorMaximum(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_BV", nil)
	sort := f.pin(f.m.BitVectorSort(8))
	name, err := f.m.Symbols.Intern("objective-query-bv")
	astTestMust(t, err)
	x := f.pin(f.m.MkConst(name, sort))
	astTestMust(t, f.c.Maximize(x))
	objectives, err := f.c.OptimizationObjectives()
	astTestMust(t, err)
	handle, err := f.m.RefGet(objectives[0])
	astTestMust(t, err)
	if kind, kindErr := f.m.GetDeclKind(handle); kindErr != nil || kind != 3 {
		t.Fatal("bit-vector negation", kind, kindErr)
	}
	argument, err := f.m.GetArg(handle, 0)
	astTestMust(t, err)
	if argument != x {
		t.Fatal("bit-vector operand", argument, x)
	}
	astTestMust(t, f.m.DelRef(&objectives[0]))
}
