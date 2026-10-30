package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicRootsValidatesBeforeInterruptScope(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	truth := f.pin(f.m.MkTrue())
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if roots, callErr := f.c.AlgebraicRoots(truth, nil); roots != nil || !errors.Is(callErr, ast.ErrASTDomain) {
		t.Fatal("algebraic roots validation order", roots, callErr)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
