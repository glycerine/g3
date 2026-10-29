package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestHornNormalizerPullsExistentialBeforeDisjunctiveSplit(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	v, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	exists := hnfNestedQuantifier(t, f, false, v, "disjunct")
	disjunction, err := f.manager.MkOr(f.app["Q"], exists)
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(disjunction, f.app["P"])
	ruleMust(t, err)
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	ruleMust(t, f.rules.AddFormula(set, formula, "nested-disjunction"))
	if count, err := set.Len(); err != nil || count != 2 {
		t.Fatal("nested disjunction split count", count, err)
	}
}
