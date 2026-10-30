package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextSortDomainSnapshotIsIndependent(t *testing.T) {
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	ruleMust(t, context.RegisterFiniteSort(boolean, base.DatalogSortSymbol))
	name, err := f.manager.Symbols.Intern("snapshot-name")
	ruleMust(t, err)
	_, err = context.SymbolNumber(boolean, name)
	ruleMust(t, err)
	snapshot, err := context.SortDomainSnapshot(boolean)
	ruleMust(t, err)
	if snapshot.Kind != base.DatalogSortSymbol || !snapshot.Finite || snapshot.DeclaredSize != 2 || len(snapshot.Symbols) != 1 || snapshot.Symbols[0] != name {
		t.Fatal("snapshot", snapshot)
	}
	other, err := f.manager.Symbols.Intern("snapshot-other")
	ruleMust(t, err)
	snapshot.Symbols[0] = other
	if got, err := context.SymbolConstant(boolean, 0); err != nil || got != name {
		t.Fatal("snapshot aliases domain", got, err)
	}
}
