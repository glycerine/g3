package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleManagerPublicValidationMatchesConstruction(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	ruleMust(t, f.rules.ValidateHead(f.head))
	ruleMust(t, f.rules.ValidateRule(f.head, []ast.ASTHandle{f.head}))
	name, err := f.manager.Symbols.Intern("unregistered-head")
	ruleMust(t, err)
	declaration, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{f.integer}, f.boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	head, err := f.manager.MkApp(declaration, []ast.ASTHandle{f.x})
	ruleMust(t, err)
	if err = f.rules.ValidateHead(head); err == nil {
		t.Fatal("unregistered predicate validated")
	}
}
