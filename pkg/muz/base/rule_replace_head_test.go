package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleReplaceHeadCopiesTailAndPreservesRequestedName(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	name, err := f.manager.Symbols.Intern("relation-Q")
	ruleMust(t, err)
	declaration, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{f.integer}, f.boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, f.rules.RegisterPredicate(declaration))
	head, err := f.manager.MkApp(declaration, []ast.ASTHandle{f.x})
	ruleMust(t, err)
	source, err := f.rules.NewRule(f.head, []ast.ASTHandle{f.head}, []bool{true}, "old")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, source.Close()) })
	result, err := f.rules.ReplaceRuleHead(source, head, "new")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, result.Close()) })
	if got, err := result.Head(); err != nil || got != head {
		t.Fatal("replacement head", got, err)
	}
	if got, err := result.Name(); err != nil || got != "new" {
		t.Fatal("replacement name", got, err)
	}
	tail, negative, err := result.Tail(0)
	ruleMust(t, err)
	if tail != f.head || !negative {
		t.Fatal("copied tail", tail, negative)
	}
}
