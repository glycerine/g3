package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2QFUFNRATacticMixedArithmetic(t *testing.T) {
	for _, tc := range []struct{ name, assertions, suffix, want string }{
		{"sat-model", "(assert (> (f x) 0))", "(get-value ((> (f x) 0)))", "sat\n(((> (f x) 0) true))\n"},
		{"nonlinear-interface-model", "(assert (> (f (+ x 1)) (* x x)))", "(get-value ((> (f (+ x 1)) (* x x))))", "sat\n(((> (f (+ x 1)) (* x x)) true))\n"},
		{"arithmetic-conflict", "(assert (> (f x) 0))(assert (< (f x) 0))", "", "unsat\n"},
		{"function-congruence", "(assert (= x y))(assert (> (f x) 0))(assert (< (f y) 0))", "", "unsat\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			script := "(set-logic QF_UFNRA)(declare-const x Real)(declare-const y Real)(declare-fun f (Real) Real)" + tc.assertions + "(check-sat-using qfufnra)" + tc.suffix
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSMT2QFUFNRATacticUnsatCore(t *testing.T) {
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
	script := `(set-option :produce-unsat-cores true)
(set-logic QF_UFNRA)
(declare-const x Real)
(declare-fun f (Real) Real)
(assert (! (> (f x) 0) :named pos))
(assert (! (< (f x) 0) :named neg))
(check-sat-using qfufnra)
(get-unsat-core)`
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	if !strings.HasPrefix(result, "unsat\n") || !strings.Contains(result, "pos") || !strings.Contains(result, "neg") {
		t.Fatal("unsat core", result)
	}
}
