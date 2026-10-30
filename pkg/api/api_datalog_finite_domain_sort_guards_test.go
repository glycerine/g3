package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFiniteDomainSortGuards(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.FiniteDomainSort("api-empty-finite", 0); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("zero finite-domain size", err)
	}
	boolean := f.pin(f.c.BoolSort())
	if _, err := f.c.FiniteDomainSortSize(boolean); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-finite sort size", err)
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err := closed.FiniteDomainSort("closed", 2); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed finite-domain constructor", err)
	}
}
