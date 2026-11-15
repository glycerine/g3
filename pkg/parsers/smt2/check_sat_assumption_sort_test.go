package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2CheckSatRejectsNonBooleanAssumptions(t *testing.T) {
	for _, command := range []string{"(check-sat x)", "(check-sat-assuming (x))"} {
		t.Run(command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			context.PrintSuccess = false
			var output bytes.Buffer
			err = ParseCommands(context, strings.NewReader("(declare-const x Int)"+command), &output)
			if err == nil || !strings.Contains(err.Error(), "argument must be a Boolean literal") || output.Len() != 0 {
				t.Fatal(err, output.String())
			}
			if closeErr := context.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
}

// AUDIT.md sat_04-F2: parse_assumptions accepts only a constant or
// (not <constant>); anything else is rejected before check_sat runs.
func TestSMT2CheckSatAssumptionsMustBeLiterals(t *testing.T) {
	for _, tc := range []struct{ command, message string }{
		{"(check-sat-assuming ((not (not p))))", "invalid check-sat command, literal expected"},
		{"(check-sat-assuming ((> a 1)))", "invalid check-sat command, 'not' expected, assumptions must be Boolean literals"},
		{"(check-sat-assuming ((and p q)))", "invalid check-sat command, 'not' expected, assumptions must be Boolean literals"},
		{"(check-sat-assuming (true))", "invalid check-sat command, argument must be a Boolean literal"},
		{"(check-sat-assuming (d))", "invalid check-sat command, argument must be a Boolean literal"},
		{"(check-sat-assuming ((not nq)))", "invalid check-sat command, argument must be a Boolean literal"},
		{"(check-sat-assuming ((not p q)))", "invalid check-sat command, ')' expected"},
		{"(check-sat (= p q))", "invalid check-sat command, 'not' expected, assumptions must be Boolean literals"},
		{"(check-sat 1)", "invalid check-sat command, literal or ')' expected"},
	} {
		t.Run(tc.command, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			context.PrintSuccess = false
			var output bytes.Buffer
			script := "(declare-const p Bool)(declare-const q Bool)(declare-const a Int)(define-fun d () Bool (and p q))(define-fun nq () Bool (not q))" + tc.command
			err = ParseCommands(context, strings.NewReader(script), &output)
			if err == nil || !strings.HasSuffix(err.Error(), tc.message) || output.Len() != 0 {
				t.Fatal(err, output.String())
			}
			if closeErr := context.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
	// A negation reached through a defined constant is a literal.
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	context.PrintSuccess = false
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader("(declare-const q Bool)(define-fun nq () Bool (not q))(check-sat-assuming (nq))(check-sat (not q))"), &output); err != nil || output.String() != "sat\nsat\n" {
		t.Fatal(err, output.String())
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
}
