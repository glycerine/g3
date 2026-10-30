package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPILabelDeclarationKindSourceAndLiveValues(t *testing.T) {
	if got := apiLabelDeclarationKind(0); got != APIDeclarationLabel {
		t.Fatal("label map", got)
	}
	if got := apiLabelDeclarationKind(1); got != APIDeclarationLabelLiteral {
		t.Fatal("label literal map", got)
	}
	if got := apiLabelDeclarationKind(2); got != APIDeclarationInternal {
		t.Fatal("unknown label operation", got)
	}
	f := newAPITypedASTFixture(t)
	name, err := f.manager.Symbols.Intern("api-label")
	if err != nil {
		t.Fatal(err)
	}
	truth, err := f.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	label, err := f.manager.MkLabel(true, []usymbol.Symbol{name}, truth)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.LabelDeclarationKind(label); err != nil || got != APIDeclarationLabel {
		t.Fatal("label app", got, err)
	}
	literal, err := f.manager.MkLabelLit([]usymbol.Symbol{name})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.LabelDeclarationKind(literal); err != nil || got != APIDeclarationLabelLiteral {
		t.Fatal("label literal", got, err)
	}
	if _, err := f.context.LabelDeclarationKind(f.application); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("uninterpreted family accepted", err)
	}
}
