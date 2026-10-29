package base_test

import (
	"github.com/glycerine/g3/pkg/muz/base"
	"strings"
	"testing"
)

func TestRuleSetReplaceRulePreservesPositionAndGrouping(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	old := f.rule("P", nil, nil)
	defer func() { ruleMust(t, old.Close()) }()
	replacement := f.rule("P", []string{"Q"}, nil)
	defer func() { ruleMust(t, replacement.Close()) }()
	ruleMust(t, set.Add(old))
	ruleMust(t, set.ReplaceRule(old, replacement))
	got, err := set.Rule(0)
	ruleMust(t, err)
	defer func() { ruleMust(t, got.Close()) }()
	equal, err := got.StructurallyEqual(replacement)
	ruleMust(t, err)
	if !equal {
		t.Fatal("replacement not stored at original position")
	}
	grouped, err := set.PredicateRules(f.decl["P"])
	ruleMust(t, err)
	defer func() {
		for _, rule := range grouped {
			ruleMust(t, rule.Close())
		}
	}()
	if len(grouped) != 1 {
		t.Fatalf("grouped replacements = %d", len(grouped))
	}
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("replacement fixture rejected")
	}
	if err = set.ReplaceRule(replacement, old); err == nil || !strings.Contains(err.Error(), "reopen") {
		t.Fatalf("finalized replacement error = %v", err)
	}
}
