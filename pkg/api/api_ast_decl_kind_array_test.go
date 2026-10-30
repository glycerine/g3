package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIArrayDeclarationKindConstArray(t *testing.T) {
	f := newAPITypedASTFixture(t)
	if _, err := f.manager.RegisterArrayPlugin(); err != nil {
		t.Fatal(err)
	}
	boolean, err := f.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	arraySort, err := f.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
	if err != nil {
		t.Fatal(err)
	}
	truth, err := f.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	constant, err := f.manager.MkConstArray(arraySort, truth)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.ArrayDeclarationKind(constant); err != nil || got != APIDeclarationConstArray {
		t.Fatal(got, err)
	}
}
