package smt2

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

// AUDIT.md parsers_shell_01-F11: the empty-constructor message, and
// check_duplicate's timing: SMT-LIB 2.6 declare-datatypes commits first
// (the sort stays declared; a duplicate inside one constructor fails in
// the commit), the legacy form checks before inserting its sorts.
func TestSMT2DeclareDatatypesDuplicateAccessors(t *testing.T) {
	for _, tc := range []struct{ script, want string }{
		{"(declare-datatypes ((A 0)) (((a (x Int)) (b (x Int)))))(declare-const y A)(check-sat)", "(error \"line 1 column 20: invalid datatype declaration, repeated accessor identifier 'x'\")\nsat\n"},
		{"(declare-datatypes ((A 0)) (((a (x Int) (x Int)))))(declare-const y A)(check-sat)", "(error \"line 1 column 51: invalid declaration, function 'x' (with the given signature) already declared\")\nsat\n"},
		{"(declare-datatypes () ((A (a (x Int)) (b (x Int)))))(declare-const y A)(check-sat)", "(error \"line 1 column 20: invalid datatype declaration, repeated accessor identifier 'x'\")\n(error \"line 1 column 70: Invalid constant declaration: unknown sort 'A'\")\nsat\n"},
		{"(declare-datatypes () ((A)))", "(error \"line 1 column 26: invalid datatype declaration, datatype does not have any constructors\")\n"},
		{"(declare-datatype A ())", "(error \"line 1 column 22: invalid datatype declaration, datatype does not have any constructors\")\n"},
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
