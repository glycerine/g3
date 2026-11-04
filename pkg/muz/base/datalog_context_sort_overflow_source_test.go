package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

// The source inserts and numbers an element before checking the finite sort
// limit. Repeated lookup of that out-of-range element still throws, while
// count and formatting observe the already-inserted element.
func TestDatalogContextFiniteSortOverflowRetainsElement(t *testing.T) {
	for _, kind := range []base.DatalogSortKind{base.DatalogSortSymbol, base.DatalogSortUint64} {
		name := "symbol"
		if kind == base.DatalogSortUint64 {
			name = "uint64"
		}
		t.Run(name, func(t *testing.T) {
			f := newRuleSetFixture(t)
			c, err := base.NewDatalogContext(f.manager)
			ruleMust(t, err)
			t.Cleanup(func() { ruleMust(t, c.Close()) })
			boolean, err := f.manager.MkBoolSort()
			ruleMust(t, err)
			ruleMust(t, c.RegisterFiniteSort(boolean, kind))
			symbols := []string{"domain-first", "domain-second", "domain-overflow"}
			for i, value := range symbols {
				var n uint64
				if kind == base.DatalogSortSymbol {
					sym, e := f.manager.Symbols.Intern(value)
					ruleMust(t, e)
					n, err = c.SymbolNumber(boolean, sym)
				} else {
					n, err = c.Uint64Number(boolean, uint64(10+i))
				}
				if i < 2 {
					if err != nil || n != uint64(i) {
						t.Fatal("in-range numbering", i, n, err)
					}
					continue
				}
				if err == nil || err.Error() != "sort Bool contains more constants than its declared size 2" {
					t.Fatal("overflow diagnostic", err)
				}
				if kind == base.DatalogSortSymbol {
					sym, e := f.manager.Symbols.Intern(value)
					ruleMust(t, e)
					_, err = c.SymbolNumber(boolean, sym)
				} else {
					_, err = c.Uint64Number(boolean, uint64(10+i))
				}
				if err == nil || err.Error() != "sort Bool contains more constants than its declared size 2" {
					t.Fatal("overflow retry diagnostic", err)
				}
			}
			count, err := c.ConstantCount(boolean)
			ruleMust(t, err)
			if count != 3 {
				t.Fatal("overflow element not retained", count)
			}
			got, err := c.FormatConstant(boolean, 2)
			ruleMust(t, err)
			want := "domain-overflow"
			if kind == base.DatalogSortUint64 {
				want = "12"
			}
			if got != want {
				t.Fatal("overflow element formatting", got, want)
			}
			if kind == base.DatalogSortSymbol {
				sym, e := f.manager.Symbols.Intern(symbols[0])
				ruleMust(t, e)
				n, e := c.SymbolNumber(boolean, sym)
				if e != nil || n != 0 {
					t.Fatal("valid lookup after overflow", n, e)
				}
			} else {
				n, e := c.Uint64Number(boolean, 10)
				if e != nil || n != 0 {
					t.Fatal("valid lookup after overflow", n, e)
				}
			}
		})
	}
}
