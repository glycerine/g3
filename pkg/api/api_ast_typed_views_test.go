package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPITypedASTViewsPreserveHandleIdentity(t *testing.T) {
	f := newAPITypedASTFixture(t)
	checks := []struct {
		name string
		want ast.ASTHandle
		call func() (ast.ASTHandle, error)
	}{
		{"application-to-ast", f.application, func() (ast.ASTHandle, error) { return f.context.ApplicationToAST(f.application) }},
		{"to-application", f.application, func() (ast.ASTHandle, error) { return f.context.ToApplication(f.application) }},
		{"sort-to-ast", f.sort, func() (ast.ASTHandle, error) { return f.context.SortToAST(f.sort) }},
		{"to-sort", f.sort, func() (ast.ASTHandle, error) { return f.context.ToSort(f.sort) }},
		{"function-to-ast", f.declaration, func() (ast.ASTHandle, error) { return f.context.FunctionDeclarationToAST(f.declaration) }},
		{"to-function", f.declaration, func() (ast.ASTHandle, error) { return f.context.ToFunctionDeclaration(f.declaration) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			got, err := check.call()
			if err != nil || got != check.want {
				t.Fatal(got, check.want, err)
			}
		})
	}
}
