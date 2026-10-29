package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type apiModelEvaluateFixture struct {
	*smtAPIFixture
	model  *APIModel
	x, yes ast.ASTHandle
}

func newAPIModelEvaluateFixture(t *testing.T) *apiModelEvaluateFixture {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("api-model-evaluate-x", boolean))
	yes := f.pin(f.c.Bool(true))
	declaration, err := f.m.GetDecl(x)
	astTestMust(t, err)
	md, err := f.c.NewAPIModel()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, md.Close()) })
	astTestMust(t, md.RegisterConstant(declaration, yes))
	return &apiModelEvaluateFixture{smtAPIFixture: f, model: md, x: x, yes: yes}
}
