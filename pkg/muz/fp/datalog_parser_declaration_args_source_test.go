package fp_test

import (
	"github.com/glycerine/g3/pkg/muz/fp"
	"testing"
)

func TestDatalogParserDeclarationArgumentBoundaries(t *testing.T) {
	for _, input := range []string{"N 2\n\nP()\n", "N 2\n\nP(x:N,)\n"} {
		t.Run(input, func(t *testing.T) {
			f := newDatalogParserFixture(t)
			if err := fp.ParseDatalogString(f.context, input); err == nil {
				t.Fatal("invalid declaration accepted")
			}
			predicates, err := f.context.Predicates()
			fpMust(t, err)
			if len(predicates) != 0 {
				t.Fatal("invalid declaration published predicate", len(predicates))
			}
		})
	}
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "N 2\n\nP\nQ(x:N y:N)\n"))
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 2 {
		t.Fatal("valid declaration count", len(predicates))
	}
}
