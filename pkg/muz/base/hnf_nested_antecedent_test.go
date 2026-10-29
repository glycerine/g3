package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestHornNormalizerDualizesExistentialInImplicationAntecedent(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	v, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	exists := hnfNestedQuantifier(t, f, false, v, "antecedent")
	formula, err := f.manager.MkImplies(exists, f.app["P"])
	ruleMust(t, err)
	hnfRequireSinglePrefix(t, f, hnfNormalizeNested(t, f, formula))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	ruleMust(t, f.rules.AddFormula(set, formula, "nested-antecedent"))
	rule, err := set.Rule(0)
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	if tails, err := rule.TailSize(); err != nil || tails != 1 {
		t.Fatal("nested antecedent tail", tails, err)
	}
}
