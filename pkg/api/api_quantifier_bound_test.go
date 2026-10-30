package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextBoundVariableSourceIndexAndSort(t *testing.T) {
	f := newQuantifierUpdateFixture(t)
	value, err := f.context.BoundVariable(41, f.sort)
	value = f.pin(t, value, err)
	if kind, err := f.manager.GetKind(value); err != nil || kind != ast.ASTVar {
		t.Fatal(kind, err)
	}
	if index, err := f.manager.GetVarIndex(value); err != nil || index != 41 {
		t.Fatal(index, err)
	}
	if sort, err := f.manager.GetSort(value); err != nil || sort != f.sort {
		t.Fatal(sort, err)
	}
	if _, err := f.context.BoundVariable(0, f.truth); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-sort accepted", err)
	}
}
