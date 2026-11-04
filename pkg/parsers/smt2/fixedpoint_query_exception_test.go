package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// Source: dl_query_cmd catches rule_properties::check_for_negated_predicates'
// default_exception, prints unknown and continues executing subsequent queries.
func TestFixedpointQueryCatchesNegativePredicateException(t *testing.T) {
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
	script := `(set-option :fixedpoint.engine spacer)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.slice false)
(declare-rel P ())
(declare-rel Q ())
(rule P)
(rule (=> (not P) Q))
(query Q)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	one := "(error \"query failed: Rule contains negative predicate <null>:\nQ() :- \n not P().\n\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
	reason, err := context.FixedpointReasonUnknown()
	if err != nil || reason != "ok" {
		t.Fatalf("status after cleanup = %q, %v", reason, err)
	}
}
