package smt2

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2CLPEngineConstrainedRules(t *testing.T) {
	for _, tc := range []struct{ name, query, want string }{
		{"zero", "G0", "unsat\n"},
		{"one", "G1", "sat\n"},
		{"existential-predicate", "Q", "sat\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := `(set-option :fixedpoint.engine clp)
(set-logic HORN)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel G0 ())
(declare-rel G1 ())
(declare-var x Int)
(rule (P 0))
(rule (P 1))
(rule (=> (and (P x) (> x 0)) (Q x)))
(rule (=> (Q 0) G0))
(rule (=> (Q 1) G1))
(query ` + tc.query + `)`
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSMT2CLPQueryDepthIncludesSyntheticRule(t *testing.T) {
	for _, tc := range []struct {
		name    string
		depth   int
		options string
		want    string
	}{
		{"boundary-sat", 18, "", "sat\n"},
		{"boundary-unknown", 19, "", "unknown\n"},
		{"above-boundary", 20, "", "unknown\n"},
		{"clp-skips-unfold", 19, "(set-option :fixedpoint.xform.unfold_rules 2)\n(set-option :fixedpoint.xform.coalesce_rules true)\n", "unknown\n"},
		{"default-inlining", 25, "default", "sat\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			var script strings.Builder
			script.WriteString("(set-option :fixedpoint.engine clp)\n")
			if tc.options != "default" {
				script.WriteString(`
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.datalog.subsumption false)
`)
				script.WriteString(tc.options)
			}
			for i := 0; i <= tc.depth; i++ {
				fmt.Fprintf(&script, "(declare-rel P%d ())\n", i)
			}
			script.WriteString("(rule P0)\n")
			for i := 0; i < tc.depth; i++ {
				fmt.Fprintf(&script, "(rule (=> P%d P%d))\n", i, i+1)
			}
			fmt.Fprintf(&script, "(query P%d)\n", tc.depth)
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script.String()), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if output.String() != tc.want {
				t.Fatalf("CLP output %q, want %q", output.String(), tc.want)
			}
		})
	}
}

func TestSMT2CLPCertificateForReachableAndUnreachable(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine clp)
(declare-rel P ())
(declare-rel U ())
(rule P)
(query P :print-certificate true :print-answer true)
(query U :print-certificate true)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if want := "sat\ntrue\ntrue\n\nunsat\ntrue\n\n"; output.String() != want {
		t.Fatalf("CLP certificate output %q, want %q", output.String(), want)
	}
}

func TestSMT2CLPBacktracksAcrossConstrainedSiblingGoals(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine clp)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.datalog.subsumption false)
(set-option :fixedpoint.xform.slice false)
(set-logic HORN)
(declare-rel P (Int))
(declare-rel R (Int))
(declare-rel Reachable ())
(declare-rel Missing ())
(declare-var x Int)
(rule (P 0))
(rule (P 1))
(rule (R 1))
(rule (=> (and (P x) (R x)) Reachable))
(rule (=> (and (P x) (R x) (> x 1)) Missing))
(query Reachable)
(query Missing)
(query Reachable)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	// The final query repeats the first after an unreachable query. This checks
	// that temporary query preprocessing does not discard the original rules.
	if want := "sat\nunsat\nsat\n"; output.String() != want {
		t.Fatalf("backtracking results %q, want %q", output.String(), want)
	}
}
