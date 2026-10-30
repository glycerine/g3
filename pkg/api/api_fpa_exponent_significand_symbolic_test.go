package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointExponentSignificandSymbolicRetention(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	exponentName := astTestSymbol(t, f.manager, "fp-symbolic-exponent")
	exponent, err := f.manager.MkConst(exponentName, f.integer)
	exponent = f.pin(t, exponent, err)
	value := f.convert(t, f.rounding, exponent, f.numeral(t, "3/2", f.real), f.single)
	if result, found := f.rewrite(t, value); found || result != (ast.ASTHandle{}) {
		t.Fatal("symbolic Int input was reduced", found, result)
	}
}
