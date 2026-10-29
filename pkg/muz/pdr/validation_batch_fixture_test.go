package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/smtcore"
)

type validationBatchFixture struct {
	command     *cmd_context.Context
	manager     *ast.ASTManager
	context     *pdr.Context
	ruleManager *base.RuleManager
	rules       *base.RuleSet
	declaration ast.ASTHandle
}

func newValidationBatchFixture(t *testing.T, name string) *validationBatchFixture {
	t.Helper()
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	manager, err := command.Manager()
	pdrMust(t, err)
	globals, err := command.GlobalParameters()
	pdrMust(t, err)
	context, err := pdr.NewContext(manager, nil, 2, pdr.ReachableConstraintCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	pdrMust(t, err)
	boolean, err := manager.MkBoolSort()
	pdrMust(t, err)
	declaration := pdrFunction(t, manager, name, nil, boolean)
	ruleManager, err := base.NewRuleManager(manager)
	pdrMust(t, err)
	pdrMust(t, ruleManager.RegisterPredicate(declaration))
	rules, err := base.NewRuleSet(ruleManager)
	pdrMust(t, err)
	head, err := manager.MkApp(declaration, nil)
	pdrMust(t, err)
	rule, err := ruleManager.NewRule(head, nil, nil, name+"-fact")
	pdrMust(t, err)
	pdrMust(t, rules.Add(rule))
	pdrMust(t, rule.Close())
	pdrMust(t, context.Initialize(rules))
	fixture := &validationBatchFixture{command, manager, context, ruleManager, rules, declaration}
	t.Cleanup(func() {
		pdrMust(t, fixture.context.Close())
		pdrMust(t, fixture.rules.Close())
		pdrMust(t, fixture.ruleManager.Close())
		pdrMust(t, fixture.command.Close())
	})
	return fixture
}
