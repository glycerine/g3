package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPublicPseudoBooleanDeclarationKinds(t *testing.T) {
	f := newAPITypedASTFixture(t)
	truth, err := f.manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	falsehood, err := f.manager.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	arguments := []ast.ASTHandle{truth, falsehood}
	applications := []struct {
		value ast.ASTHandle
		want  APIDeclarationKind
	}{}
	atMost, err := f.manager.MkPseudoBooleanAtMost(arguments, 1)
	if err != nil {
		t.Fatal(err)
	}
	applications = append(applications, struct {
		value ast.ASTHandle
		want  APIDeclarationKind
	}{atMost, APIDeclarationPBAtMost})
	atLeast, err := f.manager.MkPseudoBooleanAtLeast(arguments, 1)
	if err != nil {
		t.Fatal(err)
	}
	applications = append(applications, struct {
		value ast.ASTHandle
		want  APIDeclarationKind
	}{atLeast, APIDeclarationPBAtLeast})
	for _, test := range applications {
		got, err := f.context.CoreDeclarationKind(test.value)
		if err != nil || got != test.want {
			t.Fatal(got, test.want, err)
		}
	}
}

func TestAPIPublicPseudoBooleanKindTable(t *testing.T) {
	for operation, want := range map[int32]APIDeclarationKind{
		ast.ASTPBOpLE: APIDeclarationPBLE, ast.ASTPBOpGE: APIDeclarationPBGE,
		ast.ASTPBOpEQ: APIDeclarationPBEQ, ast.ASTPBOpAtMost: APIDeclarationPBAtMost,
		ast.ASTPBOpAtLeast: APIDeclarationPBAtLeast,
	} {
		if got := apiPseudoBooleanDeclarationKind(operation); got != want {
			t.Fatal(operation, got, want)
		}
	}
}
