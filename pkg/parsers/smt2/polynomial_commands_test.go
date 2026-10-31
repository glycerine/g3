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

func TestSMT2PolynomialFactorCommand(t *testing.T) {
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
(poly/factor (- (* x x) 1))
(poly/factor (* (+ x 1) (+ x 1)) :num-primes 2)`
	if err := ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	want := "(factors\n1\n(+ x 1.0)\n(+ x (- 1.0)))\n(factors\n1\n(^ (+ x 1.0) 2))\n"
	if got := output.String(); got != want {
		t.Fatalf("factor output = %q, want %q", got, want)
	}
}

func TestSMT2PolynomialFactorRejectsBadParameters(t *testing.T) {
	for _, script := range []string{"(poly/factor 1 :num-primes -1)", "(poly/factor 1 :unknown 1)"} {
		context, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		err = ParseCommands(context, strings.NewReader(script), &bytes.Buffer{})
		if closeErr := context.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err == nil {
			t.Fatalf("accepted %q", script)
		}
	}
}
