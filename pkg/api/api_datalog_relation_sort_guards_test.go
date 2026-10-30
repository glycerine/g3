package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextRelationSortQueryGuards(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	if _, err := f.c.RelationArity(boolean); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-relation arity", err)
	}
	relation := f.pin(f.m.MkDatalogRelationSort([]ast.ASTHandle{boolean}))
	if _, err := f.c.RelationColumn(relation, 1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("relation column bounds", err)
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err := closed.RelationColumns(relation); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed relation query", err)
	}
}
