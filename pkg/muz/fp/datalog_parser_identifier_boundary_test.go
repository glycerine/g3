package fp_test

import (
	"github.com/glycerine/g3/pkg/muz/fp"
	"testing"
)

func TestDatalogParserIdentifierPunctuation(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 4\n\nP(x!:N y<:N z>:N)\nP(X! Y< Z>).\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	manager, err := f.context.Manager()
	fpMust(t, err)
	arity, err := manager.GetArity(predicates[0])
	fpMust(t, err)
	if arity != 3 || f.ruleCount(t) != 1 {
		t.Fatal("unexpected parsed rule", arity, f.ruleCount(t))
	}
	for i, want := range []string{"x!", "y<", "z>"} {
		sym, e := f.context.ArgumentName(predicates[0], uint32(i))
		fpMust(t, e)
		got, e := manager.Symbols.Display(sym)
		fpMust(t, e)
		if got != want {
			t.Fatal("argument name", i, got, want)
		}
	}
}
