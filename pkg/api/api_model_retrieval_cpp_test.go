package api

// Source: src/test/model_retrieval.cpp tst_model_retrieval.
import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIArrayModelRetrievalCPP(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolSort := f.pin(f.c.BoolSort())
	arraySort := f.pin(f.c.ArraySort([]ast.ASTHandle{boolSort}, boolSort))
	a1 := f.pin(f.c.Constant("model-retrieval-a1", arraySort))
	a2 := f.pin(f.c.Constant("model-retrieval-a2", arraySort))
	yes := f.pin(f.m.MkTrue())
	selectA1 := f.pin(f.c.ArraySelect(a1, yes))
	assertion := f.pin(f.c.Equal(yes, selectA1))
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	defer func() { astTestMust(t, s.Close()) }()
	astTestMust(t, s.Assert(assertion))
	status, err := s.Check()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("initial check", status)
	}
	md, err := s.Model()
	astTestMust(t, err)
	defer func() { astTestMust(t, md.Close()) }()
	decl, err := f.m.GetDecl(a1)
	astTestMust(t, err)
	value, err := md.ConstantInterpretation(decl)
	astTestMust(t, err)
	if value.IsNull() {
		t.Fatal("array has no model interpretation")
	}
	valueRef, err := f.m.NewRef(value)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&valueRef)) }()
	eq := f.pin(f.c.Equal(a2, value))
	astTestMust(t, s.Assert(eq))
	status, err = s.Check()
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("direct array theory should report incompleteness for as-array", status)
	}
	reason, err := s.ReasonUnknown()
	astTestMust(t, err)
	if reason != "(incomplete (theory array))" {
		t.Fatal("unknown reason", reason)
	}
	result, err := md.Evaluate(assertion, true)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&result)) }()
	h, err := f.m.RefGet(result)
	astTestMust(t, err)
	if !f.m.IsTrue(h) {
		t.Fatal("original model no longer satisfies assertion", h)
	}
}
