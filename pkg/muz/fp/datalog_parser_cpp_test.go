package fp_test

// Source: src/test/datalog_parser.cpp tst_datalog_parser.
import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserCPPSourceInputs(t *testing.T) {
	for i, tc := range []struct {
		text     string
		accepted bool
	}{
		{"\nH :- C1(X,a,b), C2(Y,a,X) .", false},
		{"N 128\n\nH :- C1(X,a,b), C2(Y,a,X) .", false},
		{"N 128\nI 128\n\nC1(x : N, y : N, z : I)\nC2(x : N, y : N, z : N)\nH :- C1(X,a,b), C2(Y,a,X) .", true},
		{"\nH :- C1(X,a,b), nC2(Y,a,X) .", false},
		{"\nH :- C1(X,a,b),nC2(Y,a,X).", false},
		{"\nH :- C1(X,a,b),\\\nC2(Y,a,X).", false},
		{"\nH :- C1(X,a\\,\\b), C2(Y,a,X) .", false},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			f := newDatalogParserFixture(t)
			err := fp.ParseDatalogString(f.context, tc.text)
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%v, want %v: %v", err == nil, tc.accepted, err)
			}
		})
	}
}

func TestDatalogParserTypedDeclarationAsRuleHead(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nQ(x:N)\nP(x:N) :- Q(X).\n"))
	if count := f.ruleCount(t); count != 1 {
		t.Fatal("rule count", count)
	}
	rules, err := f.context.Rules()
	fpMust(t, err)
	rule, err := rules.Rule(0)
	fpMust(t, err)
	defer func() { fpMust(t, rule.Close()) }()
	head, err := rule.Head()
	fpMust(t, err)
	tail, negative, err := rule.Tail(0)
	fpMust(t, err)
	if negative {
		t.Fatal("unexpected negated tail")
	}
	m, err := f.context.Manager()
	fpMust(t, err)
	hArg, err := m.GetArg(head, 0)
	fpMust(t, err)
	tArg, err := m.GetArg(tail, 0)
	fpMust(t, err)
	if hArg != tArg {
		t.Fatal("source declaration variable was not shared with first body variable", hArg, tArg)
	}
}
