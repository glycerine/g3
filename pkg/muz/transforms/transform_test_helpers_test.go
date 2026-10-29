package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type transformPredicate struct {
	declaration ast.ASTHandle
	application ast.ASTHandle
}

func declareTransformPredicate(t *testing.T, f *coneFixture, name string, domain, arguments []ast.ASTHandle) transformPredicate {
	t.Helper()
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	symbol, err := f.manager.Symbols.Intern(name)
	coneMust(t, err)
	declaration, err := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(declaration))
	application, err := f.manager.MkApp(declaration, arguments)
	coneMust(t, err)
	return transformPredicate{declaration: declaration, application: application}
}

func addTransformRule(t *testing.T, f *coneFixture, head ast.ASTHandle, tails []ast.ASTHandle, negative []bool, name string) {
	t.Helper()
	rule, err := f.rules.NewRule(head, tails, negative, name)
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
}
