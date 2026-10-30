package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointObjectsHaveIndependentState(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	other, err := f.c.NewFixedpoint()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, other.Close()) })
	astTestMust(t, other.SetEngine("pdr"))
	otherP, err := other.DeclareRelation("api-object-fp-other-p", nil)
	astTestMust(t, err)

	astTestMust(t, f.fp.AddFact(f.p, nil))
	status, err := f.fp.Query(f.p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("first fixedpoint fact", status)
	}
	status, err = other.Query(otherP)
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("second fixedpoint observed first state", status)
	}
	otherRules, err := other.Rules()
	astTestMust(t, err)
	defer releaseAPIRefs(t, f.m, otherRules)
	if len(otherRules) != 0 {
		t.Fatal("second fixedpoint observed first rules", len(otherRules))
	}
}
