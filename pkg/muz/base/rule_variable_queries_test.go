package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleUsedVariablesAndDenseSortVector(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	y, err := f.manager.MkVar(1, f.boolean)
	ruleMust(t, err)
	constraint, err := f.manager.MkIff(y, y)
	ruleMust(t, err)
	rule, err := f.rules.NewRule(f.head, []ast.ASTHandle{constraint}, nil, "variables")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rule.Close()) })
	used, err := rule.UsedVariables()
	ruleMust(t, err)
	if used.NumVars() != 2 || used.Contains(0) != f.integer || used.Contains(1) != f.boolean {
		t.Fatal("used variables", used.NumVars(), used.Contains(0), used.Contains(1))
	}
	sorts, err := rule.VariableSorts()
	ruleMust(t, err)
	if len(sorts) != 2 || sorts[0] != f.integer || sorts[1] != f.boolean {
		t.Fatal("variable sorts", sorts)
	}
}
