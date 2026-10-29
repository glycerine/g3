package base_test

import "testing"

func TestHornNormalizerPullsThroughBooleanITEBranches(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	v, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	exists := hnfNestedQuantifier(t, f, false, v, "ite-branch")
	choice, err := f.manager.MkIte(f.app["Q"], exists, f.app["Q"])
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(choice, f.app["P"])
	ruleMust(t, err)
	hnfRequireSinglePrefix(t, f, hnfNormalizeNested(t, f, formula))
}
