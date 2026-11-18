package qe_test

// Source: src/qe/qe_arith_plugin.cpp mk_resolve: SASSERT(a.is_neg() !=
// b.is_neg()). The numeral manager's Sign is the raw small value, so the
// guard compares normalized signs.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestArithmeticQEResolveRejectsSameSignNonUnitCoefficients(t *testing.T) {
	f := newArithmeticQEFixture(t)
	resolved, handled, err := f.util.ResolveUnitOrReal(f.rx, false, smallRat(2, 1), f.ry, smallRat(3, 1), f.rx)
	if !errors.Is(err, ast.ErrASTDomain) || handled || resolved != (ast.ASTRef{}) {
		t.Fatal("same-sign coefficients 2 and 3 must be rejected", handled, err)
	}
}
