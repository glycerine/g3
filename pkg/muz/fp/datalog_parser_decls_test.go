package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserDeclarationArgumentNamesAndOutput(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 4\n\nEdge(from:N,to:N) outputtuples\n"))
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 1 {
		t.Fatal(len(predicates))
	}
	manager, err := f.context.Manager()
	fpMust(t, err)
	for i, want := range []string{"from", "to"} {
		name, e := f.context.ArgumentName(predicates[0], uint32(i))
		fpMust(t, e)
		text, e := manager.Symbols.Display(name)
		fpMust(t, e)
		if text != want {
			t.Fatalf("argument %d = %q", i, text)
		}
	}
}

func TestDatalogParserRequiresDeclarationsBeforeRules(t *testing.T) {
	f := newDatalogParserFixture(t)
	if err := fp.ParseDatalogString(f.context, "N 2\n\nP(X).\n"); err == nil {
		t.Fatal("undeclared rule head accepted")
	}
}

func TestDatalogParserZeroArityDeclarationAndFact(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nReady\nReady.\n"))
	if count := f.ruleCount(t); count != 1 {
		t.Fatal("zero-arity fact count", count)
	}
}
