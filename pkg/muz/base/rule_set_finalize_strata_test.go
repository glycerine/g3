package base_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

// AUDIT.md muz_base_02-F1: rule_set::close numbers the strata with
// rule_stratifier::process (Kahn's order from the in-degree-0 components,
// reversed), so for p1 :- e1. p2 :- e2. both e's precede both p's.
func TestRuleSetFinalizeStrataMatchSourceStratifier(t *testing.T) {
	f := newRuleSetFixture(t, "p1", "e1", "p2", "e2")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	for _, rule := range []*base.Rule{
		f.rule("p1", []string{"e1"}, nil),
		f.rule("p2", []string{"e2"}, nil),
		f.rule("e1", nil, nil),
		f.rule("e2", nil, nil),
	} {
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
	}
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("stratified fixture rejected")
	}
	strata, err := set.Strata()
	ruleMust(t, err)
	source, err := set.SourceStrata()
	ruleMust(t, err)
	if fmt.Sprint(strata) != fmt.Sprint(source) {
		t.Fatalf("Strata %v, SourceStrata %v", strata, source)
	}
	for _, name := range []string{"e1", "e2"} {
		stratum, err := set.PredicateStratum(f.decl[name])
		ruleMust(t, err)
		if stratum >= 2 {
			t.Fatal("fact predicate stratum", name, stratum)
		}
	}
	for _, name := range []string{"p1", "p2"} {
		stratum, err := set.PredicateStratum(f.decl[name])
		ruleMust(t, err)
		if stratum < 2 {
			t.Fatal("derived predicate stratum", name, stratum)
		}
	}
}
