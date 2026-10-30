package base_test

import "testing"

func TestHornNormalizerTracksNegationPolarityWhilePulling(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	v, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	// A predicate in the quantified body requires normalization instead of
	// the source already-Horn fast path for a theory-only antecedent.
	body, err := f.manager.MkAnd(v, f.app["Q"])
	ruleMust(t, err)
	forall := hnfNestedQuantifier(t, f, true, body, "negative")
	negated, err := f.manager.MkNot(forall)
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(negated, f.app["P"])
	ruleMust(t, err)
	hnfRequireSinglePrefix(t, f, hnfNormalizeNested(t, f, formula))
}
