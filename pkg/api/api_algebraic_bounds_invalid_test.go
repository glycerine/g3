package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicNumberBoundsRejectNonIrrationalValues(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	root := f.sqrt2(t)
	if yes, err := f.c.IsIrrationalAlgebraicNumber(root); err != nil || !yes {
		t.Fatal("root classification", yes, err)
	}
	if yes, err := f.c.IsIrrationalAlgebraicNumber(f.half); err != nil || yes {
		t.Fatal("rational classification", yes, err)
	}
	if _, err := f.c.AlgebraicNumberLower(f.half, 4); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("rational lower bound", err)
	}
	truth := f.pin(f.c.Bool(true))
	if _, err := f.c.AlgebraicNumberUpper(truth, 4); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("Boolean upper bound", err)
	}
}
