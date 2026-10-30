package api

import "testing"

func TestAPIFixedpointObjectPushPopRestoresRules(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddRule(f.pAtom, "outer"))
	astTestMust(t, f.fp.Push())
	astTestMust(t, f.fp.AddRule(f.qAtom, "inner"))
	rules, err := f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 2 {
		t.Fatal("scoped rules", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)
	astTestMust(t, f.fp.Pop())
	rules, err = f.fp.Rules()
	astTestMust(t, err)
	if len(rules) != 1 {
		t.Fatal("restored rules", len(rules))
	}
	releaseAPIRefs(t, f.m, rules)
}
