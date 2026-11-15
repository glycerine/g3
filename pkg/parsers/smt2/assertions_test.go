package smt2

// Source differential: src/cmd_context/basic_cmds.cpp get-assertions.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2GetAssertionsPreservesSourceTextAndScopes(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-option :interactive-mode true)(set-logic QF_UF)
(declare-const a Bool)(declare-const b Bool)
(assert (and a true))(get-assertions)
(push)(assert (! b :named n))(get-assertions)(pop)(get-assertions)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "((and a true))\n((and a true)\n (! b :named n))\n((and a true))\n"
	if output.String() != want {
		t.Fatal("get-assertions output", output.String(), "want", want)
	}
}

func TestSMT2GetAssertionsRequiresInteractiveMode(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	err = ParseCommands(context, strings.NewReader(`(assert true)(get-assertions)`), &output)
	if err == nil || !strings.Contains(err.Error(), "interactive mode") || output.Len() != 0 {
		t.Fatal("get-assertions interactive refusal", err, output.String())
	}
}

func TestSMT2NamedExpressionIsReusableAndScoped(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	context.PrintSuccess = false
	script := `(declare-const p Bool)
(push)
(assert (! p :named named-p))
(assert (not named-p))
(check-sat)
(pop)
(assert (not p))
(check-sat)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "unsat\nsat\n" {
		t.Fatal("named expression reuse and scope", output.String())
	}
}

func TestSMT2NamedExpressionRejectsFreeVariables(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	err = ParseCommands(context, strings.NewReader(`(assert (forall ((x Bool)) (! x :named open-name)))`), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid named expression, expression contains free variables") {
		t.Fatal("free named expression accepted", err)
	}
}

// AUDIT.md parsers_shell_01-F1: parse_assert rejects a non-Boolean term with
// a command error, and the assertion is dropped.
func TestSMT2AssertRejectsNonBoolean(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var out bytes.Buffer
	err = ParseShellCommands(context, strings.NewReader("(assert 0)(check-sat)"), &out)
	const message = "line 1 column 10: invalid assert command, term is not Boolean"
	if err == nil || err.Error() != message {
		t.Fatal("first parse error", err)
	}
	if want := "(error \"" + message + "\")\nsat\n"; out.String() != want {
		t.Fatal("non-Boolean assert", out.String(), want)
	}
}
