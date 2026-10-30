package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type apiTypedASTFixture struct {
	context     *SMTContext
	manager     *ast.ASTManager
	sort        ast.ASTHandle
	declaration ast.ASTHandle
	application ast.ASTHandle
}

func newAPITypedASTFixture(t *testing.T) *apiTypedASTFixture {
	t.Helper()
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := manager.Symbols.Intern("TypedU")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := manager.MkUninterpretedSort(symbol, nil)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("typed-f")
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	constant, err := manager.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	application, err := manager.MkApp(declaration, []ast.ASTHandle{constant})
	if err != nil {
		t.Fatal(err)
	}
	return &apiTypedASTFixture{context: context, manager: manager, sort: sort, declaration: declaration, application: application}
}
