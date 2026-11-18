package api

import (
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md api_01-F5: Z3_get_symbol_string of a decl or sort name gives
// the number of a numerical symbol and "" for null, not k!N / null.
func TestDeclarationNameNumericalSymbol(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean, err := f.c.BoolSort()
	astTestMust(t, err)
	fn, err := f.c.FunctionWithSymbol(usymbol.Numerical(7), nil, boolean)
	astTestMust(t, err)
	if name, err := f.c.DeclarationName(fn); err != nil || name != "7" {
		t.Fatalf("numerical decl name %q %v", name, err)
	}
	sort, err := f.c.UninterpretedSortWithSymbol(usymbol.Symbol{})
	astTestMust(t, err)
	if name, err := f.c.SortName(sort); err != nil || name != "" {
		t.Fatalf("null sort name %q %v", name, err)
	}
}
