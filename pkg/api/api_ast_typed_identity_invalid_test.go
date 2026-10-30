package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITypedASTIDsRejectWrongKinds(t *testing.T) {
	f := newAPITypedASTFixture(t)
	if _, err := f.context.SortID(f.application); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
	if _, err := f.context.FunctionDeclarationID(f.sort); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal(err)
	}
}
