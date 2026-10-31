package api

// Source: src/test/api_bug.cpp, model for a union of integer sets.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPISourceSetUnionModel(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", func(c *SMTContext) { astTestMust(t, c.SetBoolOption("model", true)) })
	integer := f.pin(f.c.IntegerSort())
	setSort := f.pin(f.c.SetSort(integer))
	empty := f.pin(f.c.EmptySet(integer))
	fortyTwo := f.pin(f.c.IntNumeral(42, integer))
	fortyThree := f.pin(f.c.IntNumeral(43, integer))
	fortyFour := f.pin(f.c.IntNumeral(44, integer))
	one := f.pin(f.c.SetAdd(empty, fortyTwo))
	two := f.pin(f.c.SetAdd(one, fortyThree))
	union := f.pin(f.c.SetUnion(one, two))
	set := f.pin(f.c.Constant("mySet", setSort))
	eq := f.pin(f.c.Equal(set, union))
	solver, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, solver.Close()) }()
	astTestMust(t, solver.Push())
	astTestMust(t, solver.Assert(eq))
	status, err := solver.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("set union check", status)
	}
	model, err := solver.Model()
	astTestMust(t, err)
	defer func() { astTestMust(t, model.Close()) }()
	for _, element := range []struct {
		value int64
		term  ast.ASTHandle
		want  lbool.LBool
	}{{42, fortyTwo, lbool.True}, {43, fortyThree, lbool.True}, {44, fortyFour, lbool.False}} {
		member := f.pin(f.c.SetMember(element.term, set))
		ref, err := model.Evaluate(member, true)
		astTestMust(t, err)
		h, err := f.m.RefGet(ref)
		astTestMust(t, err)
		got, err := f.c.BooleanValue(h)
		astTestMust(t, err)
		astTestMust(t, f.m.DelRef(&ref))
		if got != element.want {
			t.Fatalf("membership %d: %v", element.value, got)
		}
	}
}
