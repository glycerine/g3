package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIDatatypeDeclarationKindSourceTable(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{ast.ASTDatatypeConstructor, APIDeclarationDatatypeConstructor},
		{ast.ASTDatatypeRecognizer, APIDeclarationDatatypeRecognizer},
		{ast.ASTDatatypeIs, APIDeclarationDatatypeIs},
		{ast.ASTDatatypeAccessor, APIDeclarationDatatypeAccessor},
		{ast.ASTDatatypeUpdateField, APIDeclarationDatatypeUpdateField},
	}
	for _, test := range tests {
		if got := apiDatatypeDeclarationKind(test.operation); got != test.want {
			t.Fatalf("operation %d: got %d, want %d", test.operation, got, test.want)
		}
	}
	if got := apiDatatypeDeclarationKind(ast.ASTDatatypeUpdateField + 1); got != APIDeclarationInternal {
		t.Fatal("unknown datatype operation", got)
	}
}

func TestAPIDatatypeDeclarationKindLiveConstructor(t *testing.T) {
	f := newAPITypedASTFixture(t)
	name, err := f.manager.Symbols.Intern("APIKindList")
	if err != nil {
		t.Fatal(err)
	}
	constructor, err := f.manager.Symbols.Intern("api-kind-nil")
	if err != nil {
		t.Fatal(err)
	}
	recognizer, err := f.manager.Symbols.Intern("api-kind-is-nil")
	if err != nil {
		t.Fatal(err)
	}
	sorts, err := f.manager.MkDatatypes([]ast.DatatypeDefinition{{
		Name: name,
		Constructors: []ast.DatatypeConstructorDefinition{{
			Name: constructor, Recognizer: recognizer,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	constructors, err := f.manager.DatatypeConstructors(sorts[0])
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.DatatypeDeclarationKind(constructors[0]); err != nil || got != APIDeclarationDatatypeConstructor {
		t.Fatal("constructor", got, err)
	}
}
