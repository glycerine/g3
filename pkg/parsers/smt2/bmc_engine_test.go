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
		{"bounded-after-inlining", "(set-option :fixedpoint.bmc.linear_unrolling_depth 1)", "(rule A)(rule (=> A G))", "sat\n"},
		{"bounded-without-inlining", "(set-option :fixedpoint.bmc.linear_unrolling_depth 1)(set-option :fixedpoint.xform.inline_eager false)(set-option :fixedpoint.xform.inline_linear false)", "(rule A)(rule (=> A G))", "sat\n"},
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
