package hilbert

import (
	"bytes"
	"strings"
	"testing"
)

func TestSourceDiagnosticDisplay(t *testing.T) {
	b := New(nil)
	if err := b.AddEQ(rationals(2, -1), rational(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Saturate(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := b.Display(&out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, fragment := range []string{"inequalities:\n", " + 2*x1 - x2 = 3\n", "basis:\n", "->"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("display %q lacks %q", text, fragment)
		}
	}
}
