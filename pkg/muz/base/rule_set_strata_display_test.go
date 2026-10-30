package base_test

import (
	"bytes"
	"github.com/glycerine/g3/pkg/muz/base"
	"strings"
	"testing"
)

func TestRuleSetStrataAndDependencyDisplay(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	defer func() { ruleMust(t, set.Close()) }()
	for _, rule := range []*base.Rule{f.rule("P", []string{"Q"}, []bool{true}), f.rule("Q", []string{"R"}, nil), f.rule("R", nil, nil)} {
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
	}
	valid, err := set.Finalize()
	ruleMust(t, err)
	if !valid {
		t.Fatal("strata fixture rejected")
	}
	strata, err := set.Strata()
	ruleMust(t, err)
	if len(strata) != 3 || len(strata[0]) != 1 || len(strata[1]) != 1 || len(strata[2]) != 1 || strata[1][0] != f.decl["Q"] || strata[2][0] != f.decl["P"] {
		t.Fatalf("strata = %v", strata)
	}
	var out bytes.Buffer
	ruleMust(t, set.WriteDependencies(&out))
	text := out.String()
	want := "ruleset-Q -> ruleset-R\n\nruleset-P -> ruleset-Q\n\n"
	if text != want {
		t.Fatalf("dependency display = %q, want %q", text, want)
	}
	for _, edge := range []string{"ruleset-P -> ruleset-Q", "ruleset-Q -> ruleset-R"} {
		if !strings.Contains(text, edge) {
			t.Fatalf("dependency display missing %q:\n%s", edge, text)
		}
	}
	if !strings.Contains(text, "\n\n") {
		t.Fatalf("dependency strata lack separator:\n%s", text)
	}
}
