package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPublicDeclarationKindSourceFallbacks(t *testing.T) {
	f := newAPITypedASTFixture(t)
	if got, err := f.context.CoreDeclarationKind(f.declaration); err != nil || got != APIDeclarationUninterpreted {
		t.Fatal("uninterpreted declaration", got, err)
	}
	numeral, err := f.manager.ArithmeticIntegerNumeralText("7")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.CoreDeclarationKind(numeral); !errors.Is(err, ast.ErrASTDomain) || got != APIDeclarationInternal {
		t.Fatal("unported theory family was not rejected", got, err)
	}
}
