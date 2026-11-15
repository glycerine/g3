package smt2

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

// AUDIT.md parsers_shell_01-F8 (partial): the top-level '(' check,
// parse_opt_unsigned's push/pop texts, parse_ext_cmd's argument-count and
// argument-kind texts for zero-arity commands, echo, get-info and
// get-option, and the unsupported-command output order.
func TestSMT2CommandArgumentErrorsMatchCPP(t *testing.T) {
	for _, tc := range []struct{ script, want string }{
		{"(exit 3)", "(error \"line 1 column 7: invalid command, too many arguments\")\n"},
		{"(get-assertions 1)", "(error \"line 1 column 17: invalid command, too many arguments\")\n"},
		{"(get-proof 1)", "(error \"line 1 column 12: invalid command, too many arguments\")\n"},
		{"(get-unsat-core 1)", "(error \"line 1 column 17: invalid command, too many arguments\")\n"},
		{"(get-assignment 1)", "(error \"line 1 column 17: invalid command, too many arguments\")\n"},
		{"(labels 1)", "(error \"line 1 column 9: invalid command, too many arguments\")\n"},
		{"(get-option)", "(error \"line 1 column 12: invalid command, argument(s) missing\")\n"},
		{"(get-info)", "(error \"line 1 column 10: invalid command, argument(s) missing\")\n"},
		{"(push -1)", "(error \"line 1 column 7: invalid push command, value is negative.\")\n"},
		{"(push x)", "(error \"line 1 column 7: invalid push command, integer expected\")\n"},
		{"(push 5000000000)", "(error \"line 1 column 7: invalid push command, value is too big to fit in an unsigned machine integer\")\n"},
		{"(push 1 2)", "(error \"line 1 column 9: invalid push command, ')' expected\")\n"},
		{"(pop 1 2)", "(error \"line 1 column 8: invalid pop command, argument is greater than the current stack depth\")\n"},
		{"(pop x)", "(error \"line 1 column 6: invalid push command, integer expected\")\n"},
		{"foo (check-sat)", "(error \"line 1 column 1: invalid command, '(' expected\")\nsat\n"},
		{") (check-sat)", "(error \"line 1 column 1: invalid command, '(' expected\")\nsat\n"},
		{"(echo foo)", "(error \"line 1 column 7: invalid command argument, string expected\")\n"},
		{"(echo)", "(error \"line 1 column 6: invalid command, argument(s) missing\")\n"},
		{"(echo \"a\" \"b\")", "a\n(error \"line 1 column 11: invalid command, too many arguments\")\n"},
		{"(get-info 3)", "(error \"line 1 column 11: invalid command argument, keyword expected\")\n"},
		{"(get-option 3)", "(error \"line 1 column 13: invalid command argument, keyword expected\")\n"},
		{"(3)", "(error \"line 1 column 2: invalid command, symbol expected\")\n"},
		{"(echo \"a\")(exit)(echo \"b\")", "a\n"},
		{"(push 2)(pop 2)(check-sat)", "sat\n"},
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

// print_unsupported writes "unsupported" before the diagnostic line.
func TestSMT2UnsupportedCommandOutputOrder(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = c.SetDiagnosticOutput(&output); err != nil {
		t.Fatal(err)
	}
	if err = ParseShellCommands(c, strings.NewReader("(foo)"), &output); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	if want := "unsupported\n; foo line: 1 position: 5\n"; output.String() != want {
		t.Fatalf("got %q want %q", output.String(), want)
	}
}
