package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIDatalogDeclarationKindLiveRelation(t *testing.T) {
	f := newAPITypedASTFixture(t)
	name, err := f.manager.Symbols.Intern("api-datalog-column")
	if err != nil {
		t.Fatal(err)
	}
	column, err := f.manager.MkDatalogFiniteSort(name, 4)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := f.manager.MkDatalogRelationSort([]ast.ASTHandle{column})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := f.manager.MkDatalogEmpty(relation)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := f.manager.MkDatalogClone(empty)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.DatalogDeclarationKind(clone); err != nil || got != APIDeclarationRelationClone {
		t.Fatal("clone app", got, err)
	}
	declaration, err := f.manager.GetDecl(empty)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.DatalogDeclarationKind(declaration); err != nil || got != APIDeclarationRelationEmpty {
		t.Fatal("empty declaration", got, err)
	}
	if _, err := f.context.DatalogDeclarationKind(f.application); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uninterpreted family accepted", err)
	}
}
