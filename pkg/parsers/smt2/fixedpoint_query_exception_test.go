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

func TestFixedpointQueryCatchesInfiniteSortException(t *testing.T) {
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
	script := `(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.slice false)
(declare-sort U 0)
(declare-rel P (U))
(declare-var x U)
(rule (P x))
(query P)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	one := "(error \"query failed: Rule contains infinite sorts in rule <null>:\nP(#0).\n\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
	reason, err := context.FixedpointReasonUnknown()
	if err != nil || reason != "ok" {
		t.Fatalf("status after cleanup = %q, %v", reason, err)
	}
}

// Integer relations use the retained Datalog relation extension instead of
// the ordinary finite engine's infinite-sort exception path.
func TestFixedpointQueryIntegerRelationExtensionIsNotCaught(t *testing.T) {
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
	const script = `(set-option :fixedpoint.engine datalog)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.slice false)
(declare-rel P (Int))
(rule (P 1))
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "unknown\napproximated relations\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
