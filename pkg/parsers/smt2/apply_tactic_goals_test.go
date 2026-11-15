package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2ApplyTacticPrintsResultGoals(t *testing.T) {
	output, err := runApplyTacticScript(t, `(declare-const p Bool)(assert (or p false))(apply simplify)`)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.HasPrefix(output, "(goals\n") || !strings.Contains(output, "(goal\n  p\n  :precision precise :depth 1)") || !strings.HasSuffix(output, ")\n") {
		t.Fatal(output)
	}
}

func TestSMT2ApplyTacticPrintFalseSuppressesGoals(t *testing.T) {
	output, err := runApplyTacticScript(t, `(declare-const p Bool)(assert p)(apply skip :print false)`)
	if err != nil || output != "" {
		t.Fatal(err, output)
	}
}

// AUDIT.md parsers_shell_01-F9: apply and eval have no rlimit parameter;
// parametric_cmd rejects it at the keyword.
func TestSMT2ApplyEvalRejectRLimit(t *testing.T) {
	for _, tc := range []struct{ script, want string }{
		{"(apply simplify :rlimit 10)", "(error \"line 1 column 17: invalid keyword argument\")\n"},
		{"(eval 1 :rlimit 3)", "(error \"line 1 column 9: invalid keyword argument\")\n"},
	} {
		c, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err = ParseShellCommands(c, strings.NewReader(tc.script), &output); err == nil {
			t.Fatal("expected a parse error", tc.script)
		}
		if err = c.Close(); err != nil {
			t.Fatal(err)
		}
		if output.String() != tc.want {
			t.Fatalf("%s: got %q want %q", tc.script, output.String(), tc.want)
		}
	}
}
