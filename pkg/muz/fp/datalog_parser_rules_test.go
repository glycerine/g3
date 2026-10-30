package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserFactsPositiveAndNegativeRules(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 4\n\nP(x:N)\nQ(x:N)\nP(\"a\").\nQ(X) :- P(X).\nP(X) :- !Q(X).\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	if count := f.ruleCount(t); count != 3 {
		t.Fatal("rule count", count)
	}
	rules, err := f.context.Rules()
	fpMust(t, err)
	rule, err := rules.Rule(2)
	fpMust(t, err)
	defer rule.Close()
	if size, e := rule.TailSize(); e != nil || size != 1 {
		t.Fatal(size, e)
	}
	_, negative, err := rule.Tail(0)
	fpMust(t, err)
	if !negative {
		t.Fatal("negative predicate polarity lost")
	}
}

func TestDatalogParserInfixEqualityAndOrdering(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "I int\n\nP(x:I)\nQ(x:I)\nQ(X) :- P(X), X < 3.\nP(X) :- Q(X), X != 2.\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	if count := f.ruleCount(t); count != 2 {
		t.Fatal(count)
	}
}
