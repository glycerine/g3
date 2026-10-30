package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITacticDescriptionErrorsAndClosedContext(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.TacticDescription("not-a-retained-tactic"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unknown description", err)
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err = closed.TacticDescription("skip"); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed description", err)
	}
	if _, err = closed.TacticDescriptors(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed descriptors", err)
	}
}
