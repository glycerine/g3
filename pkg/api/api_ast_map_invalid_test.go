package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPublicASTMapRejectsForeignAndNullHandles(t *testing.T) {
	_, manager, mapping := newPublicASTMapFixture(t)
	value := publicASTMapAtom(t, manager, "public-map-valid-value")
	other, err := NewSMTContext()
	astTestMust(t, err)
	defer other.Close()
	foreignManager, err := other.Manager()
	astTestMust(t, err)
	foreign := publicASTMapAtom(t, foreignManager, "public-map-foreign")
	if err := mapping.Insert(foreign, value); err == nil {
		t.Fatal("accepted foreign key")
	}
	if err := mapping.Insert(ast.ASTHandle{}, value); err == nil {
		t.Fatal("accepted null key")
	}
	if err := mapping.Insert(value, ast.ASTHandle{}); err == nil {
		t.Fatal("accepted null value")
	}
}
