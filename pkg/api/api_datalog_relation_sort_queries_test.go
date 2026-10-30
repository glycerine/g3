package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextRelationSortQueries(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	left := f.pin(f.c.FiniteDomainSort("api-relation-left", 3))
	right := f.pin(f.c.FiniteDomainSort("api-relation-right", 5))
	relation := f.pin(f.m.MkDatalogRelationSort([]ast.ASTHandle{left, right}))
	if arity, err := f.c.RelationArity(relation); err != nil || arity != 2 {
		t.Fatal("relation arity", arity, err)
	}
	if column, err := f.c.RelationColumn(relation, 1); err != nil || column != right {
		t.Fatal("relation second column", column, err)
	}
	columns, err := f.c.RelationColumns(relation)
	astTestMust(t, err)
	if len(columns) != 2 || columns[0] != left || columns[1] != right {
		t.Fatal("relation columns", columns)
	}
}
