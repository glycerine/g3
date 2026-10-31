package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextASTSymbolKindsAndValues(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	number, err := f.c.SignedIntegerSymbol(37)
	astTestMust(t, err)
	text, err := f.c.StringSymbol("symbol-text")
	astTestMust(t, err)
	if kind, e := f.c.SymbolKind(number); e != nil || kind != APISymbolInteger {
		t.Fatal("integer symbol kind", kind, e)
	}
	if value, e := f.c.SymbolInteger(number); e != nil || value != 37 {
		t.Fatal("integer symbol", value, e)
	}
	if kind, e := f.c.SymbolKind(text); e != nil || kind != APISymbolString {
		t.Fatal("string symbol kind", kind, e)
	}
	if value, e := f.c.SymbolString(text); e != nil || value != "symbol-text" {
		t.Fatal("string symbol", value, e)
	}
	if _, err = f.c.SignedIntegerSymbol(-1); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("negative integer symbol", err)
	}
	if _, err = f.c.SymbolInteger(text); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("string as integer", err)
	}
}

func TestAPINumericalSymbolNativeSignedGetterAndUnsignedSpelling(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	for _, tc := range []struct {
		value  uint32
		signed int32
		text   string
	}{
		{0, 0, "0"}, {2147483647, 2147483647, "2147483647"},
		{2147483648, -2147483648, "2147483648"}, {4294967295, -1, "4294967295"},
	} {
		symbol, err := f.c.IntegerSymbol(tc.value)
		astTestMust(t, err)
		if got, err := f.c.SymbolInteger(symbol); err != nil || got != tc.signed {
			t.Fatal("native signed symbol getter", got, tc.signed, err)
		}
		if got, err := f.c.SymbolString(symbol); err != nil || got != tc.text {
			t.Fatal("native unsigned symbol spelling", got, tc.text, err)
		}
	}
}
