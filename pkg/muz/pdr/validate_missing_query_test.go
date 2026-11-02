package pdr

// Source: pdr_context.cpp solve_impl/inductive_exception validation path when
// an initialized nonempty program has no transformer for the query.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestValidatedMissingQueryUsesUNSATFinalization(t *testing.T) {
	manager, fixtureOwner, fixtureTransformer, _, closeBase := closureFixture(t)
	defer func() {
		closureMust(t, fixtureTransformer.Close())
		closureMust(t, fixtureOwner.Close())
		closeBase()
	}()
	context, err := NewContext(manager, nil, 4, ReachableNoCache, fixtureOwner.contexts.config)
	closureMust(t, err)
	defer func() { closureMust(t, context.Close()) }()
	closureMust(t, context.SetResultValidation(true))
	declaration, err := fixtureTransformer.Head()
	closureMust(t, err)
	rules, err := base.NewRuleManager(manager)
	closureMust(t, err)
	defer func() { closureMust(t, rules.Close()) }()
	closureMust(t, rules.RegisterPredicate(declaration))
	set, err := base.NewRuleSet(rules)
	closureMust(t, err)
	defer func() { closureMust(t, set.Close()) }()
	realSort, err := manager.GetDomain(declaration, 0)
	closureMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	closureMust(t, err)
	if sort, x := manager.GetSort(zero); x != nil || sort != realSort {
		t.Fatal("fixture numeral sort", sort, realSort, x)
	}
	head, err := manager.MkApp(declaration, []ast.ASTHandle{zero})
	closureMust(t, err)
	rule, err := rules.NewRule(head, nil, nil, "validated-live-fact")
	closureMust(t, err)
	closureMust(t, set.Add(rule))
	closureMust(t, rule.Close())
	closureMust(t, context.Initialize(set))
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	name, err := manager.Symbols.Intern("validated-missing-query")
	closureMust(t, err)
	missing, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	status, err := context.Solve(missing)
	closureMust(t, err)
	if status != lbool.False {
		t.Fatal("missing query did not use validated UNSAT path", status)
	}
}
