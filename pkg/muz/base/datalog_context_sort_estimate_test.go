package base_test

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextSortSizeEstimateMatchesNativeFallbacks(t *testing.T) {
	f := newRuleSetFixture(t)
	context, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	if estimate, err := context.SortSizeEstimate(boolean); err != nil || estimate != 2 {
		t.Fatal("unregistered finite", estimate, err)
	}
	ruleMust(t, context.RegisterFiniteSort(boolean, base.DatalogSortSymbol))
	name, err := f.manager.Symbols.Intern("only-assigned")
	ruleMust(t, err)
	_, err = context.SymbolNumber(boolean, name)
	ruleMust(t, err)
	if estimate, err := context.SortSizeEstimate(boolean); err != nil || estimate != 1 {
		t.Fatal("registered count, not capacity", estimate, err)
	}
	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	if estimate, err := context.SortSizeEstimate(integer); err != nil || estimate != math.MaxUint64 {
		t.Fatal("infinite fallback", estimate, err)
	}
	ruleSort, err := f.manager.MkDatalogRuleSort()
	ruleMust(t, err)
	if estimate, err := context.SortSizeEstimate(ruleSort); err != nil || estimate != 1 {
		t.Fatal("rule sort", estimate, err)
	}
}
