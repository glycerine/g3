package smt2

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type commandGlobalsReader struct {
	input *strings.Reader
	check func()
}

func (r *commandGlobalsReader) Read(p []byte) (int, error) {
	if r.check != nil {
		check := r.check
		r.check = nil
		check()
	}
	return r.input.Read(p)
}

func TestSMT2CommandSessionGlobalBindingAndRestoration(t *testing.T) {
	outer, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := outer.Close(); err != nil {
			t.Error(err)
		}
	}()
	inner, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := inner.Close(); err != nil {
			t.Error(err)
		}
	}()
	outerGlobals, err := outer.GlobalParameters()
	if err != nil {
		t.Fatal(err)
	}
	innerGlobals, err := inner.GlobalParameters()
	if err != nil {
		t.Fatal(err)
	}
	previous, err := uparams.BindProcessGlobalParamsService(innerGlobals)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := uparams.BindProcessGlobalParamsService(previous); err != nil {
			t.Error(err)
		}
	}()
	for _, script := range []string{"(exit)", "(assert )"} {
		reader := &commandGlobalsReader{input: strings.NewReader(script)}
		reader.check = func() {
			if uparams.BorrowProcessGlobalParamsService() != outerGlobals {
				t.Fatal("outer reader reached before binding")
			}
			if err := ParseCommands(inner, strings.NewReader("(exit)"), io.Discard); err != nil {
				t.Fatal(err)
			}
			if uparams.BorrowProcessGlobalParamsService() != outerGlobals {
				t.Fatal("nested parse did not restore enclosing session")
			}
		}
		err := ParseCommands(outer, reader, io.Discard)
		if (err != nil) != (script == "(assert )") {
			t.Fatal("parse result", script, err)
		}
		if uparams.BorrowProcessGlobalParamsService() != innerGlobals {
			t.Fatal("session failed to restore prior binding", script)
		}
	}
}

// AUDIT.md parsers_shell_01-F5 and cmd_context_01-F6: set-option follows
// set_option_cmd: built-in keywords dispatch by value kind with the C++
// messages, :print-warning is accepted, :expand-definitions is
// unsupported, and numerals reach gparams as rational::to_string text.
func TestSMT2SetOptionBuiltinKinds(t *testing.T) {
	warnings := diagnostic.BorrowProcessWarningService()
	enabled := warnings.Enabled()
	defer func() {
		if err := warnings.Enable(enabled); err != nil {
			t.Fatal(err)
		}
	}()
	for _, tc := range []struct{ script, want string }{
		{"(set-option :print-warning false)(echo \"ok\")", "ok\n"},
		{"(set-option :expand-definitions true)", "unsupported\n"},
		{"(set-option :print-success 3)", "(error \"line 1 column 28: option value is not a numeral\")\n"},
		{"(set-option :produce-models foo)", "(error \"line 1 column 29: invalid option value, true/false expected\")\n"},
		{"(set-option :produce-models \"x\")", "(error \"line 1 column 29: option value is not a string\")\n"},
		{"(set-option :random-seed foo)", "(error \"line 1 column 26: option value is not a symbol\")\n"},
		{"(set-option :verbosity true)", "(error \"line 1 column 24: option value is not a symbol\")\n"},
		{"(set-option :regular-output-channel 3)", "(error \"line 1 column 37: option value is not a numeral\")\n"},
		{"(set-option :random-seed 5000000000)", "(error \"line 1 column 26: option value is too big to fit in a machine integer.\")\n"},
		{"(set-option :smt.random_seed 5000000000)(get-option :smt.random_seed)", "705032704\n"},
		{"(set-option :produce-proofs true)(declare-const x Int)(set-option :produce-proofs false)", "(error \"line 1 column 83: error setting ':produce-proofs', option value cannot be modified after initialization\")\n"},
		{"(set-option :interactive-mode true)(declare-const x Int)(set-option :interactive-mode false)", "(error \"line 1 column 87: error setting ':produce-assertions', option value cannot be modified after initialization\")\n"},
		{"(set-option :smt.qi.eager_threshold 1.5)(get-option :smt.qi.eager_threshold)", "3\n"},
		{"(set-option :timeout 1.5)", "(error \"line 1 column 22: Expected values for parameter timeout is an unsigned integer. It was given argument '3/2'\")\n"},
		{"(set-option :reproducible-resource-limit 5)(get-option :rlimit)", "0\n"},
		{"(set-option)", "(error \"line 1 column 12: invalid command, argument(s) missing\")\n"},
		{"(set-option 3)", "(error \"line 1 column 13: invalid command argument, keyword expected\")\n"},
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
