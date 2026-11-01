package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2PDRSlicedProofTrace(t *testing.T) {
	for _, engine := range []string{"pdr", "qpdr"} {
		t.Run(engine, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Error(err)
				}
			}()
			script := `(set-option :fixedpoint.engine ` + engine + `)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P x))
(rule (=> (P x) Goal))
(query Goal :print-certificate true)
`
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			text := output.String()
			if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "(P ") || strings.Contains(text, "!slice!") {
				t.Fatal("expected proof over the original predicates", text)
			}
		})
	}
}
