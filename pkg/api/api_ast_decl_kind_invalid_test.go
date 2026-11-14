package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPublicDeclarationKindRejectsNonDeclarationASTs(t *testing.T) {
	f := newAPITypedASTFixture(t)
	if _, err := f.context.CoreDeclarationKind(f.sort); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, err := f.context.CoreDeclarationKind(ast.ASTHandle{}); err == nil {
		t.Fatal("null handle accepted")
	}
}

func TestAPIPublicDeclarationKindRejectsForeignAndClosedOwners(t *testing.T) {
	f := newAPITypedASTFixture(t)
	other := newAPITypedASTFixture(t)
	if ast.ASTValidateHandles { // foreign, stale or closed-owner handles are rejected only in validating builds
		if _, err := f.context.CoreDeclarationKind(other.declaration); err == nil {
			t.Fatal("foreign declaration accepted")
		}
	}
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = context.CoreDeclarationKind(f.declaration); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
