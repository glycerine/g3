package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPITypedASTSymbolsPreserveStringAndNumericalKinds(t *testing.T) {
	f := newAPITypedASTFixture(t)
	gotSort, err := f.context.SortSymbol(f.sort)
	if err != nil {
		t.Fatal(err)
	}
	if text, e := f.manager.Symbols.Display(gotSort); e != nil || text != "TypedU" {
		t.Fatal(text, e)
	}
	boolean, err := f.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	numerical := usymbol.Numerical(29)
	declaration, err := f.manager.MkFuncDecl(numerical, nil, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		t.Fatal(err)
	}
	gotDeclaration, err := f.context.DeclarationSymbol(declaration)
	if err != nil || !gotDeclaration.IsNumerical() {
		t.Fatal(gotDeclaration, err)
	}
	value, err := gotDeclaration.Number()
	if err != nil || value != 29 {
		t.Fatal(value, err)
	}
}
