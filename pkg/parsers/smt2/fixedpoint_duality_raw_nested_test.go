package smt2_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestFixedpointDualityRawRulesSkipTransformedRuleChecks(t *testing.T) {
	for _, quantify := range []bool{false, true} {
		t.Run(fmt.Sprintf("quantify_arrays=%t", quantify), func(t *testing.T) {
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
			script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.xform.quantify_arrays ` + fmt.Sprint(quantify) + `)
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
			want := "sat\nsat\n"
			if quantify {
				const one = "(error \"query failed: recursive predicate (= R S) occurs nested in the body of a rule\")\nunknown\n"
				want = one + one
			}
			if got := output.String(); got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}
