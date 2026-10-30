package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type apiFixedpointFixture struct {
	*smtAPIFixture
	fp      *APIFixedpoint
	p, q, u ast.ASTHandle
	pAtom   ast.ASTHandle
	qAtom   ast.ASTHandle
	uAtom   ast.ASTHandle
	step    ast.ASTHandle
}

func newAPIFixedpointFixture(t *testing.T) *apiFixedpointFixture {
	t.Helper()
	f := newSMTAPIFixture(t, "HORN", nil)
	fixedpoint, err := f.c.NewFixedpoint()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, fixedpoint.Close()) })
	astTestMust(t, fixedpoint.SetEngine("pdr"))
	p, err := fixedpoint.DeclareRelation("api-object-fp-p", nil)
	astTestMust(t, err)
	q, err := fixedpoint.DeclareRelation("api-object-fp-q", nil)
	astTestMust(t, err)
	u, err := fixedpoint.DeclareRelation("api-object-fp-u", nil)
	astTestMust(t, err)
	pAtom := f.pin(f.m.MkApp(p, nil))
	qAtom := f.pin(f.m.MkApp(q, nil))
	uAtom := f.pin(f.m.MkApp(u, nil))
	step := f.pin(f.m.MkImplies(pAtom, qAtom))
	return &apiFixedpointFixture{
		smtAPIFixture: f, fp: fixedpoint, p: p, q: q, u: u,
		pAtom: pAtom, qAtom: qAtom, uAtom: uAtom, step: step,
	}
}

func releaseAPIRefs(t *testing.T, manager *ast.ASTManager, values []ast.ASTRef) {
	t.Helper()
	for i := range values {
		astTestMust(t, manager.DelRef(&values[i]))
	}
}
