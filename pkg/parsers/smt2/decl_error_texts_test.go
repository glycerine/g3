package smt2

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

// AUDIT.md cmd_context_01-F2: cmd_context::mk_app's message for a failed
// overload match lists the argument sorts, the range and every declaration.
func TestSMT2NoMatchingSignatureMessage(t *testing.T) {
	for _, tc := range []struct{ script, want string }{
		{"(declare-fun g (Int) Int)(declare-fun g (Real) Int)(assert (= (g true) 2))", "(error \"line 1 column 70: unknown constant g  (Bool) \ndeclared: (declare-fun g (Int) Int) \ndeclared: (declare-fun g (Real) Int) \")\n"},
		{"(declare-fun g (Int) Int)(declare-fun g (Real) Int)(assert (= ((as g Int) true) 2))", "(error \"line 1 column 79: unknown constant g  (Bool) Int \ndeclared: (declare-fun g (Int) Int) \ndeclared: (declare-fun g (Real) Int) \")\n"},
	} {
		c, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		if err = c.SetDiagnosticOutput(io.Discard); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		_ = ParseShellCommands(c, strings.NewReader(tc.script), &output)
		if err = c.Close(); err != nil {
			t.Fatal(err)
		}
		if output.String() != tc.want {
			t.Fatalf("%s: got %q want %q", tc.script, output.String(), tc.want)
		}
	}
}
