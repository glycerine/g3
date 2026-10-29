package pdr_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func updateBatchRuleSet(t *testing.T, fixture *validationBatchFixture, declaration ast.ASTHandle, constraints ...ast.ASTHandle) *base.RuleSet {
	t.Helper()
	if declaration != fixture.declaration {
		pdrMust(t, fixture.ruleManager.RegisterPredicate(declaration))
	}
	set, err := base.NewRuleSet(fixture.ruleManager)
	pdrMust(t, err)
	head, err := fixture.manager.MkApp(declaration, nil)
	pdrMust(t, err)
	rule, err := fixture.ruleManager.NewRule(head, constraints, nil, "updated-rule")
	pdrMust(t, err)
	pdrMust(t, set.Add(rule))
	pdrMust(t, rule.Close())
	return set
}
