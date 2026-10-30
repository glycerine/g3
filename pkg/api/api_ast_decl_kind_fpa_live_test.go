package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIFloatingPointDeclarationKindLiveValues(t *testing.T) {
	f := newAPITypedASTFixture(t)
	family, err := f.manager.ExistingFloatingPointFamilyID()
	if err != nil {
		t.Fatal(err)
	}
	if family == ast.NullFamilyID {
		t.Fatal("SMT context did not register floating-point plugin")
	}
	rounding, err := f.manager.MkFamilyApp(family, ast.FPAOpRMNearestEven, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.FloatingPointDeclarationKind(rounding); err != nil || got != APIDeclarationFPARMNearestTiesToEven {
		t.Fatal("rounding app", got, err)
	}
	sort, err := f.manager.FloatingPointSort(8, 24)
	if err != nil {
		t.Fatal(err)
	}
	nan, err := f.manager.MkFPANaNOfSort(sort)
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := f.manager.GetDecl(nan)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.FloatingPointDeclarationKind(declaration); err != nil || got != APIDeclarationFPANaN {
		t.Fatal("NaN declaration", got, err)
	}
	if _, err := f.context.FloatingPointDeclarationKind(f.application); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uninterpreted family accepted", err)
	}
}
