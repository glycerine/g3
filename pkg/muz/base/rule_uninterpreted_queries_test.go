package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleManagerFindsUninterpretedNonPredicateOnlyInInterpretedTail(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	name, err := f.manager.Symbols.Intern("analysis-f")
	ruleMust(t, err)
	function, err := f.manager.MkFuncDecl(name, []ast.ASTHandle{f.integer}, f.integer, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	call, err := f.manager.MkApp(function, []ast.ASTHandle{f.x})
	ruleMust(t, err)
	constraint, err := f.manager.MkEq(call, f.x)
	ruleMust(t, err)
	rule, err := f.rules.NewRule(f.head, []ast.ASTHandle{f.head, constraint}, nil, "uf")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rule.Close()) })
	got, found, err := f.rules.UninterpretedNonPredicate(rule)
	if err != nil || !found || got != function {
		t.Fatal("uninterpreted function", got, found, err)
	}
	if found, err := f.rules.HasUninterpretedNonPredicates(rule); err != nil || !found {
		t.Fatal("uninterpreted summary", found, err)
	}
	fact, err := f.rules.NewRule(f.head, []ast.ASTHandle{f.head}, nil, "predicate-only")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, fact.Close()) })
	if _, found, err := f.rules.UninterpretedNonPredicate(fact); err != nil || found {
		t.Fatal("predicate tail treated as non-predicate UF", found, err)
	}
}
