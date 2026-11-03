package smt2_test

// Source: src/muz/fp/dl_cmds.cpp print_certificate dispatches Datalog to
// engine_base::display_certificate, which throws before the final newline.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/muz/fp"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestSMT2DatalogCertificateReportsUnsupported(t *testing.T) {
	for _, test := range []struct{ predicate, status string }{{"P", "sat"}, {"U", "unsat"}} {
		t.Run(test.status, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			context.PrintSuccess = false
			t.Cleanup(func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			})
			script := `(set-option :fixedpoint.engine datalog)
(declare-rel P ())
(declare-rel U ())
(rule P)
(query ` + test.predicate + ` :print-answer false :print-certificate true)`
			var output bytes.Buffer
			err = smt2.ParseCommands(context, strings.NewReader(script), &output)
			var reported *smt2.ScannerError
			if !errors.As(err, &reported) || reported.Message != fp.ErrPropositionalEngine.Error() || reported.Line != 5 || output.String() != test.status+"\n" {
				t.Fatal("unsupported Datalog certificate", output.String(), err)
			}
			output.Reset()
			err = smt2.ParseCommands(context, strings.NewReader("(query P :print-answer true :print-certificate false)"), &output)
			if err != nil || output.String() != "sat\ntrue\n" {
				t.Fatal("query after certificate refusal", output.String(), err)
			}
		})
	}
}
