package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTUpdateApplicationsQuantifiersAndLeaves(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	x := f.pin(f.c.Constant("update-x", boolean))
	y := f.pin(f.c.Constant("update-y", boolean))
	declaration := f.pin(f.c.Function("update-f", []ast.ASTHandle{boolean}, boolean))
	application := f.pin(f.c.Apply(declaration, x))

	updatedRef, err := f.c.UpdateTerm(application, y)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&updatedRef)) })
	updated, err := f.m.RefGet(updatedRef)
	astTestMust(t, err)
	if got, e := f.m.GetDecl(updated); e != nil || got != declaration {
		t.Fatal("updated declaration", got, declaration, e)
	}
	if got, e := f.m.GetArg(updated, 0); e != nil || got != y {
		t.Fatal("updated argument", got, y, e)
	}

	variable := f.pin(f.m.MkVar(0, boolean))
	quantifier := f.pin(f.c.Forall([]ast.ASTHandle{boolean}, []string{"q"}, variable))
	truth := f.pin(f.c.Bool(true))
	quantifierRef, err := f.c.UpdateTerm(quantifier, truth)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&quantifierRef)) })
	updatedQuantifier, err := f.m.RefGet(quantifierRef)
	astTestMust(t, err)
	if got, e := f.m.GetQuantifierBody(updatedQuantifier); e != nil || got != truth {
		t.Fatal("updated quantifier body", got, truth, e)
	}

	leafRef, err := f.c.UpdateTerm(variable, x, y)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, f.m.DelRef(&leafRef)) })
	if got, e := f.m.RefGet(leafRef); e != nil || got != variable {
		t.Fatal("unchanged leaf", got, variable, e)
	}
}

func TestSMTContextASTUpdateRejectsWrongArity(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	x := f.pin(f.c.Constant("update-arity-x", boolean))
	declaration := f.pin(f.c.Function("update-arity-f", []ast.ASTHandle{boolean}, boolean))
	application := f.pin(f.c.Apply(declaration, x))
	if _, err := f.c.UpdateTerm(application); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("application arity", err)
	}
	variable := f.pin(f.m.MkVar(0, boolean))
	quantifier := f.pin(f.c.Forall([]ast.ASTHandle{boolean}, []string{"q"}, variable))
	if _, err := f.c.UpdateTerm(quantifier); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("quantifier arity", err)
	}
}
