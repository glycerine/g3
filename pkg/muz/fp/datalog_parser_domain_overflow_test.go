package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/fp"
)

// Datalog's source finite-domain table publishes an element before reporting
// that its assigned number exceeds the declared domain size.
func TestDatalogParserRetainsOverflowingDomainConstant(t *testing.T) {
	f := newDatalogParserFixture(t)
	input := "Color 1\n\nP(x:Color)\nP(\"red\").\nP(\"blue\").\n"
	if err := fp.ParseDatalogString(f.context, input); err == nil || err.Error() != "sort Color contains more constants than its declared size 1" {
		t.Fatal("overflowing fact diagnostic", err)
	}
	predicates, err := f.context.Predicates()
	fpMust(t, err)
	if len(predicates) != 1 {
		t.Fatal("lost declared predicate", len(predicates))
	}
	manager, err := f.context.Manager()
	fpMust(t, err)
	sort, err := manager.GetDomain(predicates[0], 0)
	fpMust(t, err)
	count, err := f.context.ConstantCount(sort)
	fpMust(t, err)
	if count != 2 {
		t.Fatal("overflowing constant not retained", count)
	}
	got, err := f.context.FormatConstant(sort, 1)
	fpMust(t, err)
	if got != "blue" {
		t.Fatal("overflow constant name", got)
	}
	blue, err := manager.Symbols.Intern("blue")
	fpMust(t, err)
	if _, err = f.context.SymbolNumber(sort, blue); err == nil {
		t.Fatal("overflow lookup unexpectedly succeeded")
	}
}
