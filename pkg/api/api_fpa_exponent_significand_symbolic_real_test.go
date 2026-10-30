package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointExponentSignificandSymbolicRealRetention(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	name := astTestSymbol(t, f.manager, "fp-symbolic-significand")
	significand, err := f.manager.MkConst(name, f.real)
	significand = f.pin(t, significand, err)
	value := f.convert(t, f.rounding, f.numeral(t, "2", f.integer), significand, f.single)
	if result, found := f.rewrite(t, value); found || result != (ast.ASTHandle{}) {
		t.Fatal("symbolic Real input was reduced", found, result)
	}
}
