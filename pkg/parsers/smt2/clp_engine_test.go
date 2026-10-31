package smt2

import (
	"bytes"
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
