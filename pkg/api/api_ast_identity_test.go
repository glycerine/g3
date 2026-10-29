package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTTypedIdentityAndDomainSize(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	integer := f.pin(f.c.IntegerSort())
	if equal, err := f.c.SortEqual(boolean, boolean); err != nil || !equal {
		t.Fatal("same sort", equal, err)
	}
	if equal, err := f.c.SortEqual(boolean, integer); err != nil || equal {
		t.Fatal("different sorts", equal, err)
	}
	first := f.pin(f.c.Function("identity-f", []ast.ASTHandle{boolean}, boolean))
	second := f.pin(f.c.Function("identity-g", []ast.ASTHandle{boolean}, boolean))
	if equal, err := f.c.FunctionEqual(first, second); err != nil || equal {
		t.Fatal("different functions", equal, err)
	}
	if size, err := f.c.DeclarationDomainSize(first); err != nil || size != 1 {
		t.Fatal("domain size", size, err)
	}
	value := f.pin(f.c.Constant("identity-x", boolean))
	if got, err := f.c.ApplicationAST(value); err != nil || got != value {
		t.Fatal("application conversion", got, err)
	}
	if _, err := f.c.SortEqual(value, boolean); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("typed sort guard", err)
	}
}
