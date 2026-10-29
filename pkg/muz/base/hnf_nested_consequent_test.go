package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestHornNormalizerPullsUniversalFromImplicationConsequent(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	v, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	inner, err := f.manager.MkImplies(v, f.app["P"])
	ruleMust(t, err)
	quantified := hnfNestedQuantifier(t, f, true, inner, "consequent")
	formula, err := f.manager.MkImplies(f.app["Q"], quantified)
	ruleMust(t, err)
	normalized := hnfRequireSinglePrefix(t, f, hnfNormalizeNested(t, f, formula))
	if op, err := f.manager.GetDeclKind(normalized); err != nil || op != ast.ASTOpImplies {
		t.Fatal("normalized consequent shape", op, err)
	}
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	ruleMust(t, f.rules.AddFormula(set, formula, "nested-consequent"))
	rule, err := set.Rule(0)
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	if tails, err := rule.TailSize(); err != nil || tails != 2 {
		t.Fatal("nested consequent rule tails", tails, err)
	}
}
