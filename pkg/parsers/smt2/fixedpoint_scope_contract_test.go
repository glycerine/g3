package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestSMT2FixedpointPopAfterQuerySourceContract(t *testing.T) {
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
			var output bytes.Buffer
			parse := func(script string) error { return smt2.ParseCommands(context, strings.NewReader(script), &output) }
			script := `(set-option :fixedpoint.engine ENGINE)
(declare-rel ScopedGoal ())
(fixedpoint-push)
(rule ScopedGoal)
(query ScopedGoal :print-answer false)
`
			if err := parse(strings.Replace(script, "ENGINE", engine, 1)); err != nil {
				t.Fatal(err)
			}
			err = parse("(fixedpoint-pop)")
			if engine == "duality" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "pop operation is only supported by duality engine") {
				t.Fatal("wrong source scope restriction", err)
			}
			if err := parse("(query ScopedGoal :print-answer false)"); err != nil {
				t.Fatal(err)
			}
			want := "sat\nsat\n"
			if engine == "duality" {
				want = "sat\nunsat\n"
			}
			if output.String() != want {
				t.Fatal("pop changed the wrong query state", output.String(), want)
			}
		})
	}
}
