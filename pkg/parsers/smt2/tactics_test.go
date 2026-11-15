package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2TacticGrammarRejectsMalformedInputBeforeChecking(t *testing.T) {
	for _, text := range []string{
		"(check-sat-using)", "(check-sat-using unknown-tactic)",
		"(check-sat-using (then))", "(check-sat-using (or-else))",
		"(check-sat-using (then smt", "(check-sat-using (try-for smt))",
		"(check-sat-using (repeat smt 2 extra))", "(check-sat-using smt extra)",
		"(check-sat-using (using-params smt :max_conflicts))",
		"(check-sat-using (using-params smt :max_conflicts -1))",
		"(check-sat-using (using-params smt :max_conflicts true))",
		"(check-sat-using (using-params smt :candidate_models 1))",
		"(check-sat-using (using-params smt :unported_parameter 1))",
		"(check-sat-using (using-params qfauflia :array.simplify true))",
		"(check-sat-using (using-params qflra :max_steps 0))",
		"(check-sat-using (using-params normalize-bounds :max_steps 0))",
		"(check-sat-using (using-params reduce-bv-size :max_steps 0))",
		"(check-sat-using (using-params bv_bound_chk :max_steps 0))",
		"(check-sat-using (using-params bv_bound_chk :max_memory 0))",
		"(check-sat-using (using-params card2bv :max_steps 0))",
		"(check-sat-using (using-params reduce-bv-size :max_memory 0))",
		"(check-sat-using (using-params qflra :arith.least_error_pivot true))",
		"(check-sat-using smt :rlimit false)",
	} {
		t.Run(text, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader("(assert false)"+text), &output); err == nil {
				t.Fatal("invalid tactic grammar was accepted")
			}
			if output.Len() != 0 {
				t.Fatal("malformed tactic command ran a check", output.String())
			}
			output.Reset()
			if err = ParseCommands(context, strings.NewReader("(check-sat)"), &output); err != nil {
				t.Fatal(err)
			}
			if output.String() != "unsat\n" {
				t.Fatal("malformed tactic command changed assertions", output.String())
			}
		})
	}
}

func TestSMT2TacticCombinatorsAndTypedParameters(t *testing.T) {
	for _, expression := range []string{
		"(then smt)", "(or-else smt)", "(using-params smt)",
		"(using-params smt :restart_factor 1.5 :string_solver seq :random_seed 0.0 :auto_config false)",
		"(! smt :max_conflicts 4294967295)", "(with smt :max-conflicts 4294967295)",
		"(repeat (then simplify smt) 2)", "(fail-if-branching smt 1)",
		"(skip-if-failed smt)", "(or-else fail (then elim-and smt))",
	} {
		t.Run(expression, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := context.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			var output bytes.Buffer
			script := "(set-logic QF_UF)(declare-const a Bool)(assert a)(check-sat-using " + expression + ")(get-value (a))"
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err)
			}
			if output.String() != "sat\n((a true))\n" {
				t.Fatal("tactic grammar lost the asserted model value", output.String())
			}
		})
	}
}

// MK_BIN_PROBE: =>, - and / take exactly two arguments, as the relational
// probes do (AUDIT.md parsers_shell_01-F10).
func TestSMT2BinaryProbeArity(t *testing.T) {
	for _, probe := range []string{"(- 5 1 1)", "(/ 4 2 1)", "(=> 1 1 1)", "(implies 1)", "(=)"} {
		output, err := runApplyTacticScript(t, "(check-sat-using (if (> "+probe+" 0) smt smt))\n")
		text := output
		if err != nil {
			text += err.Error()
		}
		if !strings.Contains(text, "invalid probe expression, two arguments expected") {
			t.Fatal(probe, text)
		}
	}
	output, err := runApplyTacticScript(t, "(check-sat-using (if (> (- 5 1) 2) (fail-if 1) smt))\n")
	if err != nil || output != "unknown\n" {
		t.Fatal(output, err)
	}
}
