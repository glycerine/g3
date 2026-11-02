package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserBackslashContinuesRuleBodies(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "N 2\n\nP(x:N)\nQ(x:N)\nR(x:N)\nP(\"a\").\nQ(X) :- P(X), \\   \n P(X).\nR(X) :- Q(X), \\ # continued\n P(X).\n"
	fpMust(t, fp.ParseDatalogString(f.context, input))
	if count := f.ruleCount(t); count != 3 {
		t.Fatal("continued rule count", count)
	}
}
