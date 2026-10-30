package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleManagerClassifiesInterpretedTailQuantifiers(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	forall := f.quantified(t, true, truth)
	exists := f.quantified(t, false, truth)
	both, err := f.manager.MkAnd(forall, exists)
	ruleMust(t, err)
	rule, err := f.rules.NewRule(f.head, []ast.ASTHandle{both}, nil, "quantifiers")
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rule.Close()) })
	got, err := f.rules.Quantifiers(rule)
	if err != nil || !got.Universal || !got.Existential {
		t.Fatal("quantifier classification", got, err)
	}
	if found, err := f.rules.HasRuleQuantifiers(rule); err != nil || !found {
		t.Fatal("quantifier summary", found, err)
	}
}
