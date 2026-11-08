package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestTabSessionTransfersBorrowsAndClosesAcrossExpressionLifecycle(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Fatal(err)
		}
	})
	if err = engine.SelectEngine("tab"); err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	declare := func(text string) (ast.ASTHandle, ast.ASTHandle) {
		t.Helper()
		name, e := manager.Symbols.Intern(text)
		if e != nil {
			t.Fatal(e)
		}
		declaration, e := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		if e == nil {
			e = engine.RegisterPredicate(declaration)
		}
		application, appErr := manager.MkApp(declaration, nil)
		if e != nil || appErr != nil {
			t.Fatal(e, appErr)
		}
		return declaration, application
	}
	pDeclaration, p := declare("tab-session-lifecycle-p")
	qDeclaration, q := declare("tab-session-lifecycle-q")
	if err = engine.addRuleEager(p, "fact"); err != nil {
		t.Fatal(err)
	}
	step, err := manager.MkImplies(p, q)
	if err == nil {
		err = engine.addRuleEager(step, "step")
	}
	if err != nil {
		t.Fatal(err)
	}
	status, err := engine.QueryExpression(q)
	if err != nil || status != lbool.True || engine.tabSession == nil || engine.queryClone == nil {
		t.Fatal("first expression did not transfer its TAB session", status, err)
	}
	session := engine.tabSession
	firstClone := engine.queryClone
	if firstClone.tabSession != nil || firstClone.tabSessionBorrowed {
		t.Fatal("transferred session remained owned by first clone")
	}
	status, err = engine.QueryPredicate(qDeclaration)
	if err != nil || status != lbool.True || engine.tabSession != session {
		t.Fatal("direct query did not reuse transferred session", status, err)
	}
	status, err = engine.QueryExpression(q)
	if err != nil || status != lbool.True || engine.tabSession != session || engine.queryClone == nil {
		t.Fatal("second expression did not reuse TAB session", status, err)
	}
	secondClone := engine.queryClone
	if secondClone == firstClone || secondClone.tabSession != session || !secondClone.tabSessionBorrowed {
		t.Fatal("second expression clone did not borrow root session")
	}
	if err = engine.SelectEngine("datalog"); err != nil {
		t.Fatal(err)
	}
	if engine.tabSession != nil || engine.tabSessionBorrowed || secondClone.tabSession != nil || secondClone.tabSessionBorrowed {
		t.Fatal("backend switch left TAB session ownership attached")
	}
	// The engine remains usable after session disposal.
	status, err = engine.QueryPredicate(pDeclaration)
	if err != nil || status != lbool.True {
		t.Fatal("post-switch Datalog query", status, err)
	}
}
