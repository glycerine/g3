package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type pdrLifecycleFixture struct {
	manager      *ast.ASTManager
	engine       *PropositionalEngine
	declarations map[string]ast.ASTHandle
	applications map[string]ast.ASTHandle
}

func newPDRLifecycleFixture(t *testing.T) *pdrLifecycleFixture {
	t.Helper()
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	if err != nil {
		t.Fatal(err)
	}
	if err = engine.SelectEngine("pdr"); err != nil {
		t.Fatal(err)
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	fixture := &pdrLifecycleFixture{manager: manager, engine: engine, declarations: make(map[string]ast.ASTHandle), applications: make(map[string]ast.ASTHandle)}
	for _, label := range []string{"p", "q", "u"} {
		name, x := manager.Symbols.Intern("pdr-lifecycle-" + label)
		if x != nil {
			t.Fatal(x)
		}
		declaration, x := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		if x == nil {
			x = engine.RegisterPredicate(declaration)
		}
		application, appErr := manager.MkApp(declaration, nil)
		if x != nil || appErr != nil {
			t.Fatal(x, appErr)
		}
		fixture.declarations[label] = declaration
		fixture.applications[label] = application
	}
	if err = engine.AddRule(fixture.applications["p"], "pdr-lifecycle-fact"); err != nil {
		t.Fatal(err)
	}
	edge, err := manager.MkImplies(fixture.applications["p"], fixture.applications["q"])
	if err == nil {
		err = engine.AddRule(edge, "pdr-lifecycle-edge")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return fixture
}
