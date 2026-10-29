package base_test

import "testing"

func TestHornNormalizerPullsCompatibleBooleanEqualityArm(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	v, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	exists := hnfNestedQuantifier(t, f, false, v, "equivalence")
	equivalence, err := f.manager.MkEq(f.app["Q"], exists)
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(equivalence, f.app["P"])
	ruleMust(t, err)
	normalized := hnfNormalizeNested(t, f, formula)
	if count, err := f.manager.GetNumDeclarations(normalized); err != nil || count != 1 {
		t.Fatal("Boolean equality compatible prefix", count, err)
	}
	body, err := f.manager.GetQuantifierBody(normalized)
	ruleMust(t, err)
	has, err := f.manager.HasQuantifiers(body)
	ruleMust(t, err)
	if !has {
		t.Fatal("Boolean equality over-pulled its polarity-incompatible duplicate")
	}
}
