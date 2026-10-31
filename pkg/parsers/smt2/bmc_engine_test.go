package smt2

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2BMCEngineSearch(t *testing.T) {
	for _, tc := range []struct{ name, options, rules, want string }{
		{"linear", "", "(rule A)(rule (=> A G))", "sat\n"},
		{"bounded", "(set-option :fixedpoint.bmc.linear_unrolling_depth 0)", "(rule A)(rule (=> A G))", "unknown\n"},
		{"nonlinear", "", "(rule A)(rule B)(rule (=> (and A B) G))", "sat\n"},
		{"no-rules", "", "", "unsat\n"},
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
			script := fmt.Sprintf("(set-option :fixedpoint.engine bmc)%s(set-logic HORN)(declare-rel A ())(declare-rel B ())(declare-rel G ())%s(query G)", tc.options, tc.rules)
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
