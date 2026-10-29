package sat

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"

	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestSATJustificationSourceAllKindsAndRawGetters(t *testing.T) {
	none := satJustification{}
	if !none.None() {
		t.Fatal(none)
	}
	b := newSATBinaryJustification(3)
	if !b.Binary() {
		t.Fatal(b)
	}
	if l, e := b.Literal(); l != 3 || e != nil {
		t.Fatal(l, e)
	}
	ter := newSATTernaryJustification(8, Literal(math.MaxUint32))
	if l, e := ter.Literal2(); l != Literal(0x1fffffff) || e != nil {
		t.Fatal(l, e)
	}
	ext := newSATExternalJustification(88)
	if ext.ExternalIndex() != 88 || !ext.External() {
		t.Fatal(ext)
	}
	if b.ExternalIndex() != 3 {
		t.Fatal("source has no kind assert")
	}
	a := satStore(t, nil)
	r := satMk(t, a, satLs(0, 2), false)
	off, e := a.Offset(r)
	satOK(t, e)
	c := newSATClauseJustification(off)
	if !c.Clause() || c.ClauseOffset() != off {
		t.Fatal(c)
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	satOK(t, e)
	defer out.Close()
	for i, j := range []satJustification{none, b, ter, c, ext} {
		if i > 0 {
			satOK(t, out.AppendCString("|"))
		}
		satOK(t, j.DisplayInto(out))
	}
	text, _ := out.String()
	if text != "none|binary -1|ternary 4 -268435455|clause|external" {
		t.Fatal(text)
	}
}

func TestSATJustificationGetterAssertionsUseProcessActions(t *testing.T) {
	debug, input, outputFile := satIntegrityActionFixture(t, "CCT")
	bad := satJustification{}
	satOK(t, debug.EnableAssertions(false))
	if got, err := bad.Literal(); got != 0 || err != nil {
		t.Fatal("disabled source assertion changed getter", got, err)
	}
	satOK(t, debug.EnableAssertions(true))
	if !satStorageSourceDebug {
		for _, read := range []func() (Literal, error){bad.Literal, bad.Literal1, bad.Literal2} {
			if got, err := read(); got != 0 || err != nil {
				t.Fatal("release getter", got, err)
			}
		}
		contents, err := os.ReadFile(outputFile.Name())
		satOK(t, err)
		if len(contents) != 0 {
			t.Fatalf("release SASSERT emitted %q", contents)
		}
		return
	}
	if got, err := bad.Literal(); got != 0 || err != nil {
		t.Fatal("binary getter continuation", got, err)
	}
	if got, err := bad.Literal1(); got != 0 || err != nil {
		t.Fatal("ternary getter continuation", got, err)
	}
	var ex z3exception.DefaultException
	if _, err := bad.Literal2(); !errors.As(err, &ex) {
		t.Fatal("ternary getter T action", err)
	}
	state, err := input.State()
	satOK(t, err)
	if state.Position != 3 {
		t.Fatal("getter action count", state.Position)
	}
	contents, err := os.ReadFile(outputFile.Name())
	satOK(t, err)
	prompt := "(C)ontinue, (A)bort, (S)top, (T)hrow exception, Invoke (G)DB\n"
	assertion := func(line, condition string) string {
		return "ASSERTION VIOLATION\nFile: src/sat/sat_justification.h\nLine: " + line + "\n" + condition + "\n" + prompt
	}
	want := assertion("43", "is_binary_clause()") + assertion("46", "is_ternary_clause()") + assertion("47", "is_ternary_clause()")
	if got := string(contents); got != want {
		t.Fatalf("getter assertion records: got %q, want %q", got, want)
	}
	if strings.Count(string(contents), "ASSERTION VIOLATION") != 3 {
		t.Fatal("getter assertion count")
	}
}
