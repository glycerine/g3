package main

// Source: src/muz/pdr/pdr_context.cpp model_search::get_trace.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestFixedpointPDRBranchingTraceSourceOrder(t *testing.T) {
	for _, tc := range []struct {
		name, body, answer string
	}{
		{"forward", "(and (P x) (Q x))", "(and Goal (P 0) (Q 0) (S 0))"},
		{"reverse", "(and (Q x) (P x))", "(and Goal (Q 0) (S 0) (P 0))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := fmt.Sprintf(`
(set-option :fixedpoint.engine pdr)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-logic HORN)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel S (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P 0))
(rule (S 0))
(rule (=> (S x) (Q x)))
(rule (=> %s Goal))
(query Goal :print-answer true)
`, tc.body)
			var output bytes.Buffer
			if err := run(strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			want := "sat\n" + tc.answer + "\n"
			if canonicalOutput(output.String()) != canonicalOutput(want) {
				t.Fatalf("branching trace = %q, want %q", output.String(), want)
			}
		})
	}
}
