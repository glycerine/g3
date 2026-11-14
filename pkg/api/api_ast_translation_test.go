package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTTranslationReturnsTargetOwnedReference(t *testing.T) {
	source := newSMTAPIFixture(t, "ALL", nil)
	target, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, target.Close()) })
	astTestMust(t, target.SetLogic("ALL"))
	targetManager, err := target.Manager()
	astTestMust(t, err)

	boolean := source.pin(source.m.MkBoolSort())
	constant := source.pin(source.c.Constant("translated-constant", boolean))
	negated := source.pin(source.c.Not(constant))
	translatedRef, err := source.c.TranslateAST(negated, target)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, targetManager.DelRef(&translatedRef)) })
	translated, err := targetManager.RefGet(translatedRef)
	astTestMust(t, err)
	if kind, e := targetManager.GetKind(translated); e != nil || kind != ast.ASTApp {
		t.Fatal("translated kind", kind, e)
	}
	argument, err := targetManager.GetArg(translated, 0)
	astTestMust(t, err)
	if name, e := target.DeclarationName(fMustDecl(t, targetManager, argument)); e != nil || name != "translated-constant" {
		t.Fatal("translated constant", name, e)
	}
	if source.m.IsLiveHandle(translated) {
		t.Fatal("target provenance")
	}
	// TranslateAST has closed its temporary translator; the target ASTRef is
	// therefore the sole explicit ownership bridge tested here.
	if _, err = targetManager.GetKind(translated); err != nil {
		t.Fatal("translated result did not survive translator", err)
	}
}

func fMustDecl(t *testing.T, m *ast.ASTManager, application ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	declaration, err := m.GetDecl(application)
	astTestMust(t, err)
	return declaration
}

func TestSMTContextASTTranslationRejectsSameContext(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	value := f.pin(f.c.Bool(true))
	if _, err := f.c.TranslateAST(value, f.c); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("same context", err)
	}
	alias := &SMTContext{smtContextState: f.c.smtContextState}
	if _, err := f.c.TranslateAST(value, alias); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("aliased context", err)
	}
}
