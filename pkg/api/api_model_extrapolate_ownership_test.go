package api

import "testing"

func TestAPIModelExtrapolateResultOutlivesModelReference(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	p := apiQEBoolVariable(t, f, "extrapolate-owner-p")
	owner := apiQECheckedModel(t, f, p)
	result, err := f.c.ModelExtrapolate(owner.core, p)
	astTestMust(t, err)
	astTestMust(t, owner.core.DecRef())
	owner.core = nil
	formula, err := f.m.RefGet(result)
	astTestMust(t, err)
	if formula != p {
		t.Fatal("owned result changed", formula, p)
	}
	astTestMust(t, f.m.DelRef(&result))
}
