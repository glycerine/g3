package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIArrayDeclarationKindSourceTable(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{ast.ASTArrayOpStore, APIDeclarationArrayStore},
		{ast.ASTArrayOpSelect, APIDeclarationArraySelect},
		{ast.ASTArrayOpConst, APIDeclarationConstArray},
		{ast.ASTArrayOpDefault, APIDeclarationArrayDefault},
		{ast.ASTArrayOpMap, APIDeclarationArrayMap},
		{ast.ASTArrayOpSetUnion, APIDeclarationSetUnion},
		{ast.ASTArrayOpSetIntersect, APIDeclarationSetIntersect},
		{ast.ASTArrayOpSetDifference, APIDeclarationSetDifference},
		{ast.ASTArrayOpSetComplement, APIDeclarationSetComplement},
		{ast.ASTArrayOpSetSubset, APIDeclarationSetSubset},
		{ast.ASTArrayOpAsArray, APIDeclarationAsArray},
		{ast.ASTArrayOpExt, APIDeclarationArrayExtensionality},
	}
	for _, test := range tests {
		if got := apiArrayDeclarationKind(test.operation); got != test.want {
			t.Fatal(test.operation, got, test.want)
		}
	}
	if got := apiArrayDeclarationKind(ast.ASTArrayLastOp); got != APIDeclarationInternal {
		t.Fatal("unknown array operation", got)
	}
}
