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

func TestFixedpointQueryCatchesQuantifierExceptionAcrossQueries(t *testing.T) {
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
(declare-rel P ())
(rule (=> (not (exists ((x (_ BitVec 2))) (= x #b01))) P))
(query P)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	const one = "(error \"query failed: cannot process quantifier in rule <null>:\nP() :- \n (not (= (exists ((x (_ BitVec 2))) (= x #b01)) true)).\n\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
	reason, err := context.FixedpointReasonUnknown()
	if err != nil || reason != "ok" {
		t.Fatalf("status after cleanup = %q, %v", reason, err)
	}
}

func TestFixedpointQueryCatchesUninterpretedExceptionAcrossQueries(t *testing.T) {
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
(declare-fun f (Int) Int)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (=> (and (P x) (= (f x) 1)) (P (+ x 1))))
(rule (=> (P 1) Goal))
(query Goal)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	one := "(error \"query failed: Uninterpreted 'f' in <null>:\nP(#1) :- \n P(#0),\n (= (:var 1) (+ (:var 0) 1)),\n (= (f (:var 0)) 1).\n\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
	reason, err := context.FixedpointReasonUnknown()
	if err != nil || reason != "ok" {
		t.Fatalf("status after cleanup = %q, %v", reason, err)
	}
}

func TestFixedpointQueryCatchesUnstratifiedNegationAcrossQueries(t *testing.T) {
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
(declare-rel P ())
(declare-rel Q ())
(rule (=> (not P) Q))
(rule (=> Q P))
(query Q)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	const one = "(error \"query failed: Negation is not stratified!\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
	reason, err := context.FixedpointReasonUnknown()
	if err != nil || reason != "ok" {
		t.Fatalf("status after cleanup = %q, %v", reason, err)
	}
}

func TestFixedpointQueryCatchesDeferredIllegalHead(t *testing.T) {
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
(declare-rel P ())
(rule P)
(rule false)
(query P)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	const one = "(error \"query failed: Illegal head. The head predicate needs to be uninterpreted and registered (as recursive) false\")\nunknown\n"
	if got := output.String(); got != one+one {
		t.Fatalf("output = %q, want %q", got, one+one)
	}
}

// extract_horn wraps a non-application head in equality with true before
// check_valid_head, including Boolean variables and existential formulas.
func TestFixedpointQueryCatchesNonApplicationRuleHeads(t *testing.T) {
	for _, test := range []struct{ name, declarations, rule, head string }{
		{"variable", "(declare-var x Bool)", "x", "(= (:var 0) true)"},
		{"existential", "(declare-fun f (Bool) Bool)", "(exists ((x Bool)) (f x))", "(= (exists ((x Bool)) (f x)) true)"},
	} {
		t.Run(test.name, func(t *testing.T) {
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
` + test.declarations + `
(declare-rel P ())
(rule ` + test.rule + `)
(query P)
(query P)`
			var output bytes.Buffer
			if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			one := "(error \"query failed: Illegal head. The head predicate needs to be uninterpreted and registered (as recursive) " + test.head + "\")\nunknown\n"
			if got := output.String(); got != one+one {
				t.Fatalf("output = %q, want %q", got, one+one)
			}
		})
	}
}

func TestFixedpointDeferredIllegalHeadRemovedByPop(t *testing.T) {
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
(declare-rel P ())
(rule P)
(fixedpoint-push)
(rule false)
(fixedpoint-pop)
(query P)`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "sat\n" {
		t.Fatalf("output = %q", got)
	}
}

// These engines check the rules before their default transformation closes the
// context, so the negative-predicate diagnostic precedes stratification.
func TestFixedpointQueryNegativeRulePrecedesStratification(t *testing.T) {
	for _, engine := range []string{"pdr", "spacer", "bmc", "clp"} {
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
(rule (=> (not P) Q))
(rule (=> Q P))
(query Q)
(query P)`
			var output bytes.Buffer
			if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			const one = "(error \"query failed: Rule contains negative predicate <null>:\nQ() :- \n not P().\n\")\nunknown\n"
			if got := output.String(); got != one+one {
				t.Fatalf("output = %q, want %q", got, one+one)
			}
		})
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
