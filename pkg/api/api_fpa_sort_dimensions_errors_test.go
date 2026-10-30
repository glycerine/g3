package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointSortDimensionErrors(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	boolean, err := f.manager.MkBoolSort()
	boolean = f.pin(t, boolean, err)
	if _, err = f.context.FloatingPointExponentBits(boolean); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("Boolean exponent width accepted", err)
	}
	truth, err := f.manager.MkTrue()
	truth = f.pin(t, truth, err)
	if _, err = f.context.FloatingPointSignificandBits(truth); err == nil {
		t.Fatal("expression accepted as floating-point sort")
	}
}
