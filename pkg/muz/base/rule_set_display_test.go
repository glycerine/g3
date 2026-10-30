package base_test

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleSetWriteSourceDisplay(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	for _, rule := range []*base.Rule{
		f.rule("P", nil, nil),
		f.rule("Q", nil, nil),
		f.rule("P", []string{"Q"}, []bool{false}),
	} {
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
	}
	ruleMust(t, set.SetOutputPredicate(f.decl["Q"]))
	ruleMust(t, set.SetOutputPredicate(f.decl["P"]))
	var out bytes.Buffer
	ruleMust(t, set.WriteDisplay(&out))
	want := "; rule count: 3\n; predicate count: 2\n" +
		"; output: ruleset-Q\n; output: ruleset-P\n" +
		"P:\nruleset-P().\n" +
		"P:\nruleset-P() :- \n ruleset-Q().\n" +
		"Q:\nruleset-Q().\n"
	if got := out.String(); got != want {
		t.Fatalf("rule-set display = %q, want %q", got, want)
	}
}
