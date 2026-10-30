package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointExponentSignificandDeclaration(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	value := f.convert(t, f.rounding, f.numeral(t, "2", f.integer), f.numeral(t, "3/2", f.real), f.single)
	if kind, err := f.manager.GetDeclKind(value); err != nil || kind != ast.FPAOpToFP {
		t.Fatal(kind, err)
	}
	if count, err := f.manager.GetNumArgs(value); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	if sort, err := f.manager.GetSort(value); err != nil || sort != f.single {
		t.Fatal(sort, err)
	}
}
