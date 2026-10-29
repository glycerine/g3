package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIModelEvaluateCompletionRegistersMissingConstant(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	missing := f.pin(f.c.Constant("api-model-eval-missing", boolean))
	declaration, err := f.m.GetDecl(missing)
	astTestMust(t, err)
	md, err := f.c.NewAPIModel()
	astTestMust(t, err)
	defer func() { astTestMust(t, md.Close()) }()
	if found, e := md.HasInterpretation(declaration); e != nil || found {
		t.Fatal("unexpected initial interpretation", found, e)
	}
	value, err := md.Evaluate(missing, true)
	astTestMust(t, err)
	if found, e := md.HasInterpretation(declaration); e != nil || !found {
		t.Fatal("completion did not register interpretation", found, e)
	}
	got, err := f.m.RefGet(value)
	astTestMust(t, err)
	if kind, e := f.m.GetKind(got); e != nil || kind != ast.ASTApp {
		t.Fatal("completed value", kind, e)
	}
	astTestMust(t, f.m.DelRef(&value))
}
