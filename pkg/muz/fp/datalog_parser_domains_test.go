package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

func TestDatalogParserFiniteAndIntegerDomains(t *testing.T) {
	f := newDatalogParserFixture(t)
	fpMust(t, fp.ParseDatalogString(f.context, "Node123 3\nI int\n\nP(x:Node)\nQ(i:I)\n"))
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 2 {
		t.Fatal("predicate count", len(predicates))
	}
	manager, err := f.context.Manager()
	fpMust(t, err)
	finite, err := manager.GetDomain(predicates[0], 0)
	fpMust(t, err)
	size, found, err := manager.DatalogFiniteSortSize(finite)
	fpMust(t, err)
	if !found || size != 3 {
		t.Fatal("finite declared size", size, found)
	}
	estimate, err := f.context.SortSizeEstimate(finite)
	fpMust(t, err)
	if estimate != 0 {
		t.Fatal("finite assigned-constant estimate", estimate)
	}
}

func TestDatalogParserRejectsDuplicateAndZeroDomains(t *testing.T) {
	for _, input := range []string{"N 0\n\n", "N 2\nN 2\n\n"} {
		f := newDatalogParserFixture(t)
		if err := fp.ParseDatalogString(f.context, input); err == nil {
			t.Fatal("accepted invalid domains", input)
		}
	}
}
