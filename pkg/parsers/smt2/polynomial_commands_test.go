package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2ToPolynomialCommand(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	var output bytes.Buffer
	script := `(declare-const x Real)
(to-poly (+ (* x x) 1))
(to-poly (+ x x))`
	if err := ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "(+ (^ x 2.0) 1.0)\n(* 2.0 x)\n"; got != want {
		t.Fatalf("polynomial output = %q, want %q", got, want)
	}
}

func TestSMT2ToPolynomialRejectsNonArithmetic(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	err = ParseCommands(context, strings.NewReader("(to-poly true)"), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "expression is not a polynomial") {
		t.Fatal("non-arithmetic expression accepted", err)
	}
}
