package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestFixedpointExistentialTailExceptionAcrossQueries(t *testing.T) {
	for _, engine := range []string{"pdr", "spacer", "clp", "tab", "qbmc"} {
		t.Run(engine, func(t *testing.T) {
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
			script := `(set-option :fixedpoint.engine ` + engine + `)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.slice false)
(declare-rel P ())
(declare-rel Q ())
(declare-rel R ())
(rule P)
(rule (=> (= P Q) R))
(query R)
(query P)`
			var output bytes.Buffer
			if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			const one = "(error \"query failed: recursive predicate (= P Q) occurs nested in the body of a rule\")\nunknown\n"
			if got := output.String(); got != one+one {
				t.Fatalf("output = %q, want %q", got, one+one)
			}
			reason, err := context.FixedpointReasonUnknown()
			if err != nil || reason != "ok" {
				t.Fatalf("status after cleanup = %q, %v", reason, err)
			}
		})
	}
}

func TestFixedpointExistentialTailUsesSharedRuleOrder(t *testing.T) {
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
	const script = `(set-option :fixedpoint.engine pdr)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.slice false)
(declare-rel P ())
(declare-rel Q ())
(declare-rel R ())
(declare-rel S ())
(declare-rel A ())
(declare-rel B ())
(rule (=> (= P Q) A))
(rule (=> (= R S) B))
(query A)
(query B)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	const one = "(error \"query failed: recursive predicate (= R S) occurs nested in the body of a rule\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
}
