package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextSortDomainQueries(t *testing.T) {
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	if found, err := context.HasSortDomain(boolean); err != nil || found {
		t.Fatal("unregistered domain", found, err)
	}
	if count, found, err := context.TrySortConstantCount(boolean); err != nil || found || count != 0 {
		t.Fatal("unregistered count", count, found, err)
	}
	ruleMust(t, context.RegisterFiniteSort(boolean, base.DatalogSortSymbol))
	if found, err := context.HasSortDomain(boolean); err != nil || !found {
		t.Fatal("registered domain", found, err)
	}
	if kind, err := context.SortDomainKind(boolean); err != nil || kind != base.DatalogSortSymbol {
		t.Fatal("domain kind", kind, err)
	}
}
