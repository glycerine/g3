package base_test

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextFiniteSortDomains(t *testing.T) {
	f := newRuleSetFixture(t)
	c, err := base.NewDatalogContext(f.manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, c.Close()) })
	b, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	ruleMust(t, c.RegisterFiniteSort(b, base.DatalogSortSymbol))
	left, err := f.manager.Symbols.Intern("left")
	ruleMust(t, err)
	right, err := f.manager.Symbols.Intern("right")
	ruleMust(t, err)
	extra, err := f.manager.Symbols.Intern("extra")
	ruleMust(t, err)
	if n, e := c.SymbolNumber(b, left); e != nil || n != 0 {
		t.Fatalf("left number = %d,%v", n, e)
	}
	if n, e := c.SymbolNumber(b, right); e != nil || n != 1 {
		t.Fatalf("right number = %d,%v", n, e)
	}
	if n, e := c.SymbolNumber(b, left); e != nil || n != 0 {
		t.Fatalf("repeat number = %d,%v", n, e)
	}
	if _, e := c.SymbolNumber(b, extra); e == nil || !strings.Contains(e.Error(), "exhausted") {
		t.Fatalf("finite overflow = %v", e)
	}
	if text, e := c.FormatConstant(b, 1); e != nil || text != "right" {
		t.Fatalf("constant text = %q,%v", text, e)
	}
	if count, e := c.ConstantCount(b); e != nil || count != 2 {
		t.Fatalf("constant count = %d,%v", count, e)
	}
}
