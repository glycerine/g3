package fp_test

import (
	"github.com/glycerine/g3/pkg/muz/fp"
	"testing"
)

// Source read_bid maps the decoded text through reserved_symbols, so quoted
// underscores are fresh wildcards, not occurrences of a shared variable.
func TestDatalogParserQuotedUnderscoresRemainWildcards(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nP(x:N,y:N)\nP(|_|,|_|).\n"))
	rules, err := f.context.Rules()
	fpMust(t, err)
	rule, err := rules.Rule(0)
	fpMust(t, err)
	defer func() { fpMust(t, rule.Close()) }()
	head, err := rule.Head()
	fpMust(t, err)
	manager, err := f.context.Manager()
	fpMust(t, err)
	a, err := manager.GetArg(head, 0)
	fpMust(t, err)
	b, err := manager.GetArg(head, 1)
	fpMust(t, err)
	left, err := manager.GetVarIndex(a)
	fpMust(t, err)
	right, err := manager.GetVarIndex(b)
	fpMust(t, err)
	if left == right {
		t.Fatal("quoted wildcards were unified", left, right)
	}
}
