package api

import (
	"bytes"
	"errors"
	"testing"
)

func TestZ3LogSymbolAndArrayRecords(t *testing.T) {
	var output bytes.Buffer
	log, _ := NewZ3LogWriter(&output)
	operations := []func() error{
		func() error { return log.Symbol(NullZ3LogSymbol()) },
		func() error { return log.Symbol(NumericalZ3LogSymbol(123)) },
		func() error { return log.Symbol(NamedZ3LogSymbol("x|y")) },
		func() error { return log.PointerArray(2) },
		func() error { return log.SymbolArray(3) },
		func() error { return log.UintArray(4) },
	}
	for _, operation := range operations {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	want := "N\n# 123\n$ |x\\124y|\np 2\ns 3\nu 4\n"
	if output.String() != want {
		t.Fatalf("got %q, want %q", output.String(), want)
	}
	if err := log.Symbol(Z3LogSymbol{Kind: 99}); !errors.Is(err, ErrZ3LogSymbol) {
		t.Fatalf("got %v", err)
	}
}
