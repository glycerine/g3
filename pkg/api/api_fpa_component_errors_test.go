package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointComponentAndStringErrors(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	truth, err := f.manager.MkTrue()
	truth = f.pin(t, truth, err)
	if _, err = f.context.FloatingPointNumeralComponents(false, 0, 0, truth); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-floating-point sort accepted", err)
	}
	nan, err := f.context.FloatingPointNaN(f.single)
	nan = f.pin(t, nan, err)
	if _, err = f.context.FloatingPointNumeralSignificandString(nan); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("NaN accepted", err)
	}
	name := astTestSymbol(t, f.manager, "symbolic-fp")
	symbolic, err := f.manager.MkConst(name, f.single)
	symbolic = f.pin(t, symbolic, err)
	if _, err = f.context.FloatingPointNumeralExponentString(symbolic, false); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("symbolic floating-point expression accepted", err)
	}
}
