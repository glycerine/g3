package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextSortDomainTypedReverseValues(t *testing.T) {
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	ruleMust(t, context.RegisterFiniteSort(boolean, base.DatalogSortSymbol))
	left, err := f.manager.Symbols.Intern("domain-left")
	ruleMust(t, err)
	_, err = context.SymbolNumber(boolean, left)
	ruleMust(t, err)
	if got, err := context.SymbolConstant(boolean, 0); err != nil || got != left {
		t.Fatal("symbol reverse", got, err)
	}
	symbols, err := context.SymbolConstants(boolean)
	ruleMust(t, err)
	other, err := f.manager.Symbols.Intern("domain-other")
	ruleMust(t, err)
	symbols[0] = other
	if got, err := context.SymbolConstant(boolean, 0); err != nil || got != left {
		t.Fatal("symbol copy aliases domain", got, err)
	}
	name, err := f.manager.Symbols.Intern("uint-domain")
	ruleMust(t, err)
	sort, err := f.manager.MkDatalogFiniteSort(name, 3)
	ruleMust(t, err)
	ruleMust(t, context.RegisterFiniteSort(sort, base.DatalogSortUint64))
	_, err = context.Uint64Number(sort, 99)
	ruleMust(t, err)
	if got, err := context.Uint64Constant(sort, 0); err != nil || got != 99 {
		t.Fatal("uint reverse", got, err)
	}
}
