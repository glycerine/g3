package api

import (
	"strings"
	"testing"
)

func TestSMTContextASTPrinterModesAndConfiguration(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	value := f.pin(f.c.Constant("printer-value", boolean))
	printer, err := f.c.NewASTPrinter(APIASTPrintSMTFull)
	astTestMust(t, err)
	for _, mode := range []APIASTPrintMode{APIASTPrintSMTFull, APIASTPrintLowLevel, APIASTPrintSMT2} {
		astTestMust(t, printer.SetMode(mode))
		text, e := printer.String(value)
		astTestMust(t, e)
		if text == "" {
			t.Fatal("empty AST rendering", mode)
		}
		if mode != APIASTPrintLowLevel && !strings.Contains(text, "printer-value") {
			t.Fatal("rendering lacks declaration", mode, text)
		}
	}
}
