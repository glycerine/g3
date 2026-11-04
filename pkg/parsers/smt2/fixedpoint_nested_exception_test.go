package smt2_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

// The HNF pipeline retains interpreted nested predicate expressions for
// Datalog's source check_nested_free, including when proofs are generated.
func TestFixedpointNestedPredicateExceptionAcrossQueries(t *testing.T) {
	for _, doc := range []bool{false, true} {
		for _, proof := range []bool{false, true} {
			t.Run(fmt.Sprintf("doc=%t/proof=%t", doc, proof), func(t *testing.T) {
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
				options := ""
				if doc {
					options += "(set-option :fixedpoint.datalog.default_relation doc)\n"
				}
				if proof {
					options += "(set-option :fixedpoint.generate_proof_trace true)\n"
				}
				script := `(set-option :fixedpoint.engine datalog)
` + options + `(set-option :fixedpoint.xform.inline_eager false)
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
				proofText := ""
				if proof {
					proofText = "(asserted (=> (= P Q) R))\n"
				}
				one := "(error \"query failed: Rule contains nested predicates <null>:\nR() :- \n (= P Q).\n" + proofText + "\")\nunknown\n"
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
}
