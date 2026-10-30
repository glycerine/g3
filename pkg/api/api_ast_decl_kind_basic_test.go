package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPublicBasicDeclarationKindTable(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{ast.ASTOpTrue, APIDeclarationTrue}, {ast.ASTOpFalse, APIDeclarationFalse},
		{ast.ASTOpEq, APIDeclarationEqual}, {ast.ASTOpDistinct, APIDeclarationDistinct},
		{ast.ASTOpIte, APIDeclarationIte}, {ast.ASTOpAnd, APIDeclarationAnd},
		{ast.ASTOpOr, APIDeclarationOr}, {ast.ASTOpIff, APIDeclarationIff},
		{ast.ASTOpXor, APIDeclarationXor}, {ast.ASTOpNot, APIDeclarationNot},
		{ast.ASTOpImplies, APIDeclarationImplies}, {ast.ASTOpOEq, APIDeclarationOEq},
		{ast.ASTOpInterp, APIDeclarationInterpolant},
	}
	for _, test := range tests {
		if got := apiBasicDeclarationKind(test.operation); got != test.want {
			t.Fatalf("operation %d: got %d want %d", test.operation, got, test.want)
		}
	}
}

func TestAPIPublicDeclarationKindAcceptsDeclarationAndApplication(t *testing.T) {
	f := newAPITypedASTFixture(t)
	truth, err := f.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := f.manager.GetDecl(truth)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []ast.ASTHandle{truth, declaration} {
		kind, err := f.context.CoreDeclarationKind(value)
		if err != nil || kind != APIDeclarationTrue {
			t.Fatal(kind, err)
		}
	}
}
