package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestSMT2FixedpointNonlinearIndependentIntegerBranches(t *testing.T) {
	for _, engine := range []string{"pdr", "spacer", "duality"} {
		for _, test := range []struct{ target, want string }{{"8", "sat\n"}, {"7", "unsat\n"}} {
			t.Run(engine+"/right="+test.target, func(t *testing.T) {
				context, err := cmd_context.NewStandaloneContext()
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := context.Close(); err != nil {
						t.Fatal(err)
					}
				}()
				context.PrintSuccess = false
				script := `(set-option :fixedpoint.engine ENGINE)
(declare-rel Left (Int))
(declare-rel Right (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (Left 0))
(rule (Right 10))
(rule (=> (and (Left x) (< x 2)) (Left (+ x 1))))
(rule (=> (and (Right y) (> y 8)) (Right (- y 1))))
(rule (=> (and (Left 2) (Right RIGHT_TARGET)) Goal))
(query Goal :print-answer false)
`
				script = strings.Replace(script, "ENGINE", engine, 1)
				script = strings.Replace(script, "RIGHT_TARGET", test.target, 1)
				var output bytes.Buffer
				if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
					t.Fatal(err)
				}
				if output.String() != test.want {
					t.Fatal("nonlinear recursive branches", output.String())
				}
			})
		}
	}
}

func TestSMT2FixedpointRepeatedNonlinearQueries(t *testing.T) {
	for _, engine := range []string{"pdr", "spacer", "duality"} {
		t.Run(engine, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			context.PrintSuccess = false
			script := `(set-option :fixedpoint.engine ENGINE)
(declare-rel Left (Int))
(declare-rel Right (Int))
(declare-rel Reachable ())
(declare-rel Unreachable ())
(declare-var x Int)
(declare-var y Int)
(rule (Left 0))
(rule (Right 10))
(rule (=> (and (Left x) (< x 2)) (Left (+ x 1))))
(rule (=> (and (Right y) (> y 8)) (Right (- y 1))))
(rule (=> (and (Left 2) (Right 8)) Reachable))
(rule (=> (and (Left 2) (Right 7)) Unreachable))
(query Unreachable :print-answer false)
(query Reachable :print-answer false)
(query Unreachable :print-answer false)
`
			script = strings.Replace(script, "ENGINE", engine, 1)
			var output bytes.Buffer
			if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if output.String() != "unsat\nsat\nunsat\n" {
				t.Fatal("repeated nonlinear query isolation", output.String())
			}
		})
	}
}
