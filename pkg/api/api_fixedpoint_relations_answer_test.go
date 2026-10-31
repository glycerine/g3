package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIFixedpointDatalogMultipleRelationAnswers(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.c.SetFixedpointEngine("datalog"))
	astTestMust(t, f.fp.AddFact(f.p, nil))
	status, err := f.fp.QueryRelations([]ast.ASTHandle{f.p, f.q})
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("status", status)
	}
	answer, err := f.fp.LastAnswer()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&answer)) }()
	h, err := f.m.RefGet(answer)
	astTestMust(t, err)
	yes := f.pin(f.m.MkTrue())
	no := f.pin(f.m.MkFalse())
	want := f.pin(f.m.MkAndArray([]ast.ASTHandle{yes, no}))
	if h != want {
		t.Fatal("answer", h, want)
	}
	astTestMust(t, f.fp.AddFact(f.q, nil))
	if _, err = f.fp.LastAnswer(); err == nil {
		t.Fatal("stale answer")
	}
	status, err = f.fp.QueryRelations(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("empty query", status)
	}
	empty, err := f.fp.LastAnswer()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&empty)) }()
	h, err = f.m.RefGet(empty)
	astTestMust(t, err)
	if !f.m.IsTrue(h) {
		t.Fatal("empty query answer", h)
	}
}
