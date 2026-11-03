package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserSourceConstantIdentity(t *testing.T) {
	for _, test := range []struct {
		name, fact, operand string
		identical           bool
	}{
		{"numeric-argument", "0007", "7", true},
		{"negative-zero-argument", "-000", "0", true},
		{"negative-zero-infix-spelling", "-0", "-0", false},
		{"infix-spelling", "0007", "0007", false},
		{"quoted-argument", `"0007"`, "0007", true},
		{"negative-finite-symbol", `"-3"`, "-3", true},
		{"wide-finite-symbol", `"18446744073709551616"`, "18446744073709551616", true},
		{"quoted-variable-name", `"a"`, `"X"`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			input := "N 2\n\nP(x:N)\nQ(x:N)\nP(" + test.fact + ").\nQ(X) :- P(X), X = " + test.operand + ".\n"
			fpMust(t, fp.ParseDatalogString(f.context, input))
			m, err := f.context.Manager()
			fpMust(t, err)
			rules, err := f.context.Rules()
			fpMust(t, err)
			fact, err := rules.Rule(0)
			fpMust(t, err)
			defer func() { fpMust(t, fact.Close()) }()
			rule, err := rules.Rule(1)
			fpMust(t, err)
			defer func() { fpMust(t, rule.Close()) }()
			head, err := fact.Head()
			fpMust(t, err)
			value, err := m.GetArg(head, 0)
			fpMust(t, err)
			guard, negative, err := rule.Tail(1)
			fpMust(t, err)
			if negative {
				t.Fatal("unexpected negative equality")
			}
			constant, err := m.GetArg(guard, 1)
			fpMust(t, err)
			if (value == constant) != test.identical {
				t.Fatal("constant spelling changed equality", value, constant, test.identical)
			}
		})
	}
}

func TestDatalogParserInfixRequiresIdentifierVariables(t *testing.T) {
	for _, guard := range []string{`"X" = "a"`, `"a" = "X"`, "X = Missing"} {
		t.Run(guard, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			input := "N 3\n\nP(x:N)\nQ(x:N)\nQ(X) :- P(X), " + guard + ".\n"
			if err := fp.ParseDatalogString(f.context, input); err == nil {
				t.Fatal("infix accepted a quoted or unknown variable", guard)
			}
		})
	}
}
