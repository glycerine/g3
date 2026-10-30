package base_test

import "testing"

func TestRuleManagerContainsPredicateNestedAndAbsent(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	nested, err := f.manager.MkIff(f.head, truth)
	ruleMust(t, err)
	if found, err := f.rules.ContainsPredicate(nested); err != nil || !found {
		t.Fatal("nested predicate", found, err)
	}
	absent, err := f.manager.MkEq(f.x, f.x)
	ruleMust(t, err)
	if found, err := f.rules.ContainsPredicate(absent); err != nil || found {
		t.Fatal("predicate-free expression", found, err)
	}
}
