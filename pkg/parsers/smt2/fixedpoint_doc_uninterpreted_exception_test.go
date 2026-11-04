package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestFixedpointDocUninterpretedExceptionAndQuantifierPrecedence(t *testing.T) {
	for _, test := range []struct{ name, extra, expected string }{
		{"uninterpreted", "", "Uninterpreted 'f' in <null>:\nP() :- \n (= (f #b01) #b01).\n"},
		{"quantifier-first", "(rule (=> (not (exists ((x (_ BitVec 2))) (= x #b01))) Q))", "cannot process quantifier in rule <null>:\nQ() :- \n (not (= (exists ((x (_ BitVec 2))) (= x #b01)) true)).\n"},
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
(set-option :fixedpoint.datalog.default_relation doc)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.slice false)
(declare-fun f ((_ BitVec 2)) (_ BitVec 2))
(declare-rel P ())
(declare-rel Q ())
(rule (=> (= (f #b01) #b01) P))
` + test.extra + `
(query P)
(query Q)`
			var output bytes.Buffer
			if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			one := "(error \"query failed: " + test.expected + "\")\nunknown\n"
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
