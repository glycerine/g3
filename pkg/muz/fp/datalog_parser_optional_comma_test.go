package fp_test

import (
	"github.com/glycerine/g3/pkg/muz/fp"
	"testing"
)

// Source parse_args only consumes comma when present; it starts the next
// argument directly when another argument token follows.
func TestDatalogParserArgumentsAllowSourceOptionalCommas(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 4\n\nP(x:N y:N)\nP(\"a\" \"b\").\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 1 {
		t.Fatal("predicate count", len(predicates))
	}
	manager, err := f.context.Manager()
	fpMust(t, err)
	arity, err := manager.GetArity(predicates[0])
	fpMust(t, err)
	if arity != 2 || f.ruleCount(t) != 1 {
		t.Fatal("unexpected parsed signature or fact", arity, f.ruleCount(t))
	}
	rules, err := f.context.Rules()
	fpMust(t, err)
	rule, err := rules.Rule(0)
	fpMust(t, err)
	defer func() { fpMust(t, rule.Close()) }()
	head, err := rule.Head()
	fpMust(t, err)
	for i := uint32(0); i < 2; i++ {
		arg, e := manager.GetArg(head, i)
		fpMust(t, e)
		sort, e := manager.GetSort(arg)
		fpMust(t, e)
		want := []string{"a", "b"}[i]
		got, e := f.context.FormatConstant(sort, uint64(i))
		fpMust(t, e)
		if got != want {
			t.Fatal("constant order", i, got, want)
		}
	}
}

func TestDatalogParserOptionalCommasStillEnforceArity(t *testing.T) {
	f := newDatalogParserFixture(t)
	err := fp.ParseDatalogString(f.context, "N 4\n\nP(x:N)\nP(\"a\" \"b\").\n")
	if err == nil {
		t.Fatal("extra argument accepted")
	}
	if count := f.ruleCount(t); count != 0 {
		t.Fatal("invalid fact published", count)
	}
}
