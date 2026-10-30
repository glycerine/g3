package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ruleAnalysisFixture struct {
	manager *ast.ASTManager
	rules   *base.RuleManager
	boolean ast.ASTHandle
	integer ast.ASTHandle
	x       ast.ASTHandle
	head    ast.ASTHandle
	pred    ast.ASTHandle
}

func newRuleAnalysisFixture(t *testing.T) *ruleAnalysisFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	manager, err := context.Manager()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rules.Close()) })
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	x, err := manager.MkVar(0, integer)
	ruleMust(t, err)
	name, err := manager.Symbols.Intern("analysis-P")
	ruleMust(t, err)
	pred, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, rules.RegisterPredicate(pred))
	head, err := manager.MkApp(pred, []ast.ASTHandle{x})
	ruleMust(t, err)
	return &ruleAnalysisFixture{manager: manager, rules: rules, boolean: boolean, integer: integer, x: x, head: head, pred: pred}
}

func (f *ruleAnalysisFixture) quantified(t *testing.T, forall bool, body ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	name, err := f.manager.Symbols.Intern("analysis-bound")
	ruleMust(t, err)
	quantifier, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: forall, Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: body,
	})
	ruleMust(t, err)
	return quantifier
}

func TestRuleAnalysisWalkerDescendsQuantifierBodies(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	quantifier := f.quantified(t, true, f.head)
	found, err := f.rules.ContainsPredicate(quantifier)
	if err != nil || !found {
		t.Fatal("predicate hidden in quantifier body", found, err)
	}
}
