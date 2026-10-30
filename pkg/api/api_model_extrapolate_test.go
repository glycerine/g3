package api

import "testing"

func TestAPIModelExtrapolateReturnsSatisfiedLiteral(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "extrapolate-p")
	owner := apiQECheckedModel(t, f, p)
	result, err := f.c.ModelExtrapolate(owner.core, p)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&result)) }()
	formula, err := f.m.RefGet(result)
	astTestMust(t, err)
	if formula != p {
		t.Fatal("single literal implicant changed", formula, p)
	}
}
