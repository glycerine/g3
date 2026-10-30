package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleManagerForallQuantifierDirectAndTruthIff(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	quantifier := f.quantified(t, true, f.head)
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	left, err := f.manager.MkIff(quantifier, truth)
	ruleMust(t, err)
	right, err := f.manager.MkIff(truth, quantifier)
	ruleMust(t, err)
	for _, root := range []ast.ASTHandle{quantifier, left, right} {
		got, yes, getErr := f.rules.ForallQuantifier(root)
		if getErr != nil || !yes || got != quantifier {
			t.Fatal("forall recognition", got, yes, getErr)
		}
	}
}
