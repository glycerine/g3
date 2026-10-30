package api

import "testing"

func TestAPIOptimizationObjectivesNormalizeArithmeticDirection(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("objective-query-x", true)
	astTestMust(t, f.c.Minimize(x))
	astTestMust(t, f.c.Maximize(x))
	objectives, err := f.c.OptimizationObjectives()
	astTestMust(t, err)
	if len(objectives) != 2 {
		t.Fatal("objective count", len(objectives))
	}
	first, err := f.m.RefGet(objectives[0])
	astTestMust(t, err)
	if first != x {
		t.Fatal("minimize expression changed", first, x)
	}
	second, err := f.m.RefGet(objectives[1])
	astTestMust(t, err)
	if kind, kindErr := f.m.GetDeclKind(second); kindErr != nil || kind != 8 {
		t.Fatal("maximize arithmetic normalization", kind, kindErr)
	}
	argument, err := f.m.GetArg(second, 0)
	astTestMust(t, err)
	if argument != x {
		t.Fatal("maximize operand", argument, x)
	}
	for i := range objectives {
		astTestMust(t, f.m.DelRef(&objectives[i]))
	}
}
