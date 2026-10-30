package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIDeclarationFamilyClassifiersRejectOtherFamiliesAndKinds(t *testing.T) {
	f := newAPITypedASTFixture(t)
	if _, err := f.context.ArithmeticDeclarationKind(f.declaration); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uninterpreted accepted as arithmetic", err)
	}
	if _, err := f.context.ArrayDeclarationKind(f.declaration); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uninterpreted accepted as array", err)
	}
	if _, err := f.context.ArithmeticDeclarationKind(f.sort); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("sort accepted as declaration", err)
	}
	if _, err := f.context.ArrayDeclarationKind(ast.ASTHandle{}); err == nil {
		t.Fatal("null handle accepted")
	}
}

func TestAPIDeclarationFamilyClassifiersRejectForeignAndClosedOwners(t *testing.T) {
	f := newAPITypedASTFixture(t)
	other := newAPITypedASTFixture(t)
	if _, err := f.context.ArrayDeclarationKind(other.application); err == nil {
		t.Fatal("foreign application accepted")
	}
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = context.ArithmeticDeclarationKind(f.application); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
