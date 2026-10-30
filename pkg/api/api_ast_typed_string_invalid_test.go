package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITypedASTStringFormsRejectKindsAndModes(t *testing.T) {
	f := newAPITypedASTFixture(t)
	if _, err := f.context.SortString(f.application, APIASTPrintSMT2); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, err := f.context.FunctionString(f.sort, APIASTPrintSMT2); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, err := f.context.ASTString(f.application, APIASTPrintMode(255)); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
}

func TestAPITypedASTStringFormsRejectClosedContext(t *testing.T) {
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = context.ASTString(truth, APIASTPrintSMT2); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
