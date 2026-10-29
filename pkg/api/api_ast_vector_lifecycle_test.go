package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestASTVectorLifecycleGuards(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, c.Close())
	if _, err = c.NewASTVector(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatalf("closed context: %v", err)
	}
	f, _, _ := astVectorFixture(t)
	v, err := f.c.NewASTVector()
	astTestMust(t, err)
	astTestMust(t, v.Close())
	astTestMust(t, v.Close())
	if _, err = v.Len(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("closed vector: %v", err)
	}
	if err = v.Push(ast.ASTHandle{}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("closed push: %v", err)
	}
}
