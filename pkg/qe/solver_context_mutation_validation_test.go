package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestSolverContextMutationValidationAndClose(t *testing.T) {
	f := newSolverMutationFixture(t)
	if err := f.context.AddVariable(f.base.integer); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("sort variable error=%v", err)
	}
	if err := f.context.EliminateVariable(9, f.form, ast.ASTHandle{}); !errors.Is(err, arena.ErrBounds) {
		t.Fatalf("index error=%v", err)
	}
	if err := f.context.EliminateVariable(0, f.base.integer, ast.ASTHandle{}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("sort formula error=%v", err)
	}
	contains, err := f.context.Contains(0)
	fmMust(t, err)
	fmMust(t, f.context.Close())
	if _, err = contains.Target(); err == nil {
		t.Fatal("context close retained contains owner")
	}
	if _, err = f.context.Variables(); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed variables error=%v", err)
	}
}
