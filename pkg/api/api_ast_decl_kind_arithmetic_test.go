package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIArithmeticDeclarationKindNumeralAndAddition(t *testing.T) {
	f := newAPITypedASTFixture(t)
	one, err := f.manager.ArithmeticIntegerNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	two, err := f.manager.ArithmeticIntegerNumeralText("2")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.ArithmeticDeclarationKind(one); err != nil || got != APIDeclarationArithmeticNumeral {
		t.Fatal("numeral", got, err)
	}
	family, err := f.manager.ArithmeticFamilyID()
	if err != nil {
		t.Fatal(err)
	}
	sum, err := f.manager.MkFamilyApp(family, 6, nil, []ast.ASTHandle{one, two})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.ArithmeticDeclarationKind(sum); err != nil || got != APIDeclarationAdd {
		t.Fatal("addition", got, err)
	}
}
