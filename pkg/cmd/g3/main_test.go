package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSMTLibRunnerQFIDLBranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "qfidl")
}

func TestSMTLibRunnerQFLIABranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "qflia")
}

func TestSMTLibRunnerQFBVBranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "qfbv")
}

func TestSMTLibRunnerQFAUFBVBranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "qfaufbv")
}

func TestSMTLibRunnerQFUFBVBranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "qfufbv")
}

func TestSMTLibRunnerQFUFBVAckrBranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "qfufbv_ackr")
}

func TestSMTLibRunnerQuantifiedArithmeticBranchEvidence(t *testing.T) {
	testArithmeticStrategyBranchEvidence(t, "quant")
}

func TestSMTLibRunnerAUFLIAExpressionLimit(t *testing.T) {
	// The quantified body contributes four nodes and each distinct ground
	// application contributes two. MBQI disabled with auto-config disabled
	// makes the first SMT attempt inconclusive, exposing the fallback count.
	for _, n := range []int{62, 63} {
		t.Run(fmt.Sprint(4+2*n), func(t *testing.T) {
			var script strings.Builder
			script.WriteString("(set-logic AUFLIA)(declare-fun p (Int) Bool)(assert (forall ((x Int)) (! (p x) :pattern ((p x)))))\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&script, "(assert (p %d))\n", i)
			}
			script.WriteString("(check-sat-using (using-params auflia :auto_config false :mbqi false))\n")
			level, err := diagnostic.ProcessVerbosityLevel()
			if err != nil {
				t.Fatal(err)
			}
			previous, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				t.Fatal(err)
			}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := diagnostic.SetProcessVerboseStream(previous); err != nil {
					t.Error(err)
				}
				if err := diagnostic.SetProcessVerbosityLevel(level); err != nil {
					t.Error(err)
				}
				if err := stream.Close(); err != nil {
					t.Error(err)
				}
			})
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerbosityLevel(10); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(strings.NewReader(script.String()), &out); err != nil {
				t.Fatal(err, out.String())
			}
			verbose, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			want := 2
			if n == 63 {
				want = 1
			}
			check := func(origin, output, log string) {
				t.Helper()
				if canonicalOutput(output) != "unknown" {
					t.Fatal(origin, "unexpected result", output)
				}
				if got := strings.Count(log, "(smt.tactic start)"); got != want {
					t.Fatalf("%s: %d SMT attempts, want %d\n%s", origin, got, want, log)
				}
				if !strings.Contains(log, fmt.Sprintf("(simplifier :num-exprs %d ", 4+2*n)) {
					t.Fatal(origin, "fixture lost expression boundary", log)
				}
			}
			check("Go", out.String(), verbose)
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				var native, log bytes.Buffer
				cmd := exec.Command(reference, "-v:10", "-in")
				cmd.Stdin = strings.NewReader(script.String())
				cmd.Stdout = &native
				cmd.Stderr = &log
				if err := cmd.Run(); err != nil {
					t.Fatal(err, native.String(), log.String())
				}
				check("C++", native.String(), log.String())
			}
		})
	}
}

func testArithmeticStrategyBranchEvidence(t *testing.T, prefix string) {
	t.Helper()
	cases := []struct {
		name            string
		require, forbid []string
	}{
		{"quant_auflia_core.smt2", []string{"(ctx-simplify ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"quant_uflra_proof.smt2", []string{"(ctx-simplify ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"quant_uflra_models_scopes.smt2", []string{"(ctx-simplify ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"quant_auflia_models_scopes.smt2", []string{"(ctx-simplify ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"quant_auflira_models_scopes.smt2", []string{"(ctx-simplify ", "(solve_eqs ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"quant_aufnira_models_scopes.smt2", []string{"(ctx-simplify ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"quant_ufnia_models_scopes.smt2", []string{"(ctx-simplify ", "(qe-lite ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"quant_ufnia_core.smt2", []string{"(ctx-simplify ", "(qe-lite ", "(smt.tactic start)"}, []string{"(solve_eqs ", "(sat-status"}},
		{"qfufbv_ackr_proof_refusal.smt2", []string{"(qfufbv_ackr ", "(smt.tactic start)"}, []string{"(bv-bound-chk ", "(bv-size-reduction ", "(sat-status"}},
		{"qfufbv_ackr_core_refusal.smt2", []string{"(qfufbv_ackr ", "(smt.tactic start)"}, []string{"(bv-bound-chk ", "(bv-size-reduction ", "(sat-status"}},
		{"qfufbv_ackr_smt_fallback.smt2", []string{"(smt.tactic start)"}, []string{"(qfufbv_ackr ", "(sat-status"}},
		{"qfufbv_ackr_smt_eager.smt2", []string{"(qfufbv_ackr ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfufbv_ackr_smt_lazy.smt2", []string{"(qfufbv_ackr ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfufbv_ackr_sat_eager.smt2", []string{"(qfufbv_ackr ", "(bit-blaster ", "(sat-status"}, nil},
		{"qfufbv_ackr_sat_lazy.smt2", []string{"(qfufbv_ackr ", "(ackermannize ", "(bv1-blaster ", "(smt.tactic start)"}, []string{"(card2bv "}},
		{"qfufbv_ackr_incsat_eager.smt2", []string{"(qfufbv_ackr ", "(card2bv ", "(bit-blaster "}, []string{"(sat-status", "(smt.tactic start)"}},
		{"qfufbv_ackr_incsat_lazy.smt2", []string{"(qfufbv_ackr ", "(card2bv ", "(bit-blaster "}, []string{"(sat-status", "(smt.tactic start)"}},
		{"qfufbv_smt_models_scopes.smt2", []string{"(reduce-args ", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfufbv_solver_smt_models_scopes.smt2", []string{"(reduce-args ", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfufbv_reduce_models_scopes.smt2", []string{"(:reduced-funcs 1)", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfufbv_solver_reduce_models_scopes.smt2", []string{"(:reduced-funcs 1)", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfufbv_sat_models_scopes.smt2", []string{"(:reduced-funcs 1)", "(bit-blaster ", "(aig ", "(sat-status"}, nil},
		{"qfufbv_solver_sat_models_scopes.smt2", []string{"(:reduced-funcs 1)", "(bit-blaster ", "(aig ", "(sat-status"}, nil},
		{"qfufbv_proof.smt2", []string{"(smt.tactic start)"}, []string{"(reduce-args ", "(bv-size-reduction ", "(sat-status"}},
		{"qfufbv_core.smt2", []string{"(smt.tactic start)"}, []string{"(reduce-args ", "(bv-size-reduction ", "(sat-status"}},
		{"qfaufbv_models_scopes.smt2", []string{"(bv-size-reduction ", "(max-bv-sharing ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfaufbv_solver_models_scopes.smt2", []string{"(bv-size-reduction ", "(max-bv-sharing ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfaufbv_extensional_models.smt2", []string{"(bv-size-reduction ", "(max-bv-sharing ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfaufbv_core.smt2", []string{"(max-bv-sharing ", "(smt.tactic start)"}, []string{"(bv-size-reduction ", "(sat-status"}},
		{"qfaufbv_proof.smt2", []string{"(max-bv-sharing ", "(smt.tactic start)"}, []string{"(bv-size-reduction ", "(sat-status"}},
		{"qfbv_sat_models_scopes.smt2", []string{"(ackermannize ", "(bit-blaster ", "(aig ", "(sat-status"}, []string{"(bv1-blaster "}},
		{"qfbv_solver_sat_models_scopes.smt2", []string{"(ackermannize ", "(bit-blaster ", "(aig ", "(sat-status"}, []string{"(bv1-blaster "}},
		{"qfbv_equality_models.smt2", []string{"(bv1-blaster ", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfbv_solver_equality_models.smt2", []string{"(bv1-blaster ", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfbv_uf_models.smt2", []string{"(ackermannize ", "(bit-blaster ", "(aig ", "(sat-status"}, []string{"(bv1-blaster "}},
		{"qfbv_div0_fallback.smt2", []string{"(ackermannize ", "(smt.tactic start)"}, []string{"(bit-blaster ", "(sat-status"}},
		{"qfbv_core.smt2", []string{"(bit-blaster "}, []string{"(ackermannize ", "(bv-size-reduction "}},
		{"qfbv_proof.smt2", []string{"(bit-blaster "}, []string{"(ackermannize ", "(bv-size-reduction ", "(aig ", "(sat-status"}},
		{"qflia_solver_pb_sat_pipeline.smt2", []string{"(pb-tactic start)", "(pb2bv ", "(bit-blaster ", "(sat-status"}, nil},
		{"qflia_solver_quasi_pb.smt2", []string{"(lia2sat-tactic start)", "(lia2pb ", "(pb2bv "}, []string{"(bounded-tactic start)"}},
		{"qflia_solver_bounded.smt2", []string{"(bounded-tactic start)", "(no-cut-smt-tactic start)", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qflia_solver_unbounded_ilp.smt2", []string{"(ilp-model-finder-tactic start)", "(no-cut-smt-tactic start)", "(smt.tactic start)"}, []string{"(pb-tactic start)", "(sat-status"}},
		{"qflia_solver_unbounded_disjunction.smt2", []string{"(bounded-tactic start)", "(smt.tactic start)"}, []string{"(no-cut-smt-tactic start)", "(sat-status"}},
		{"qflia_solver_core_guard.smt2", nil, []string{"(sat-status"}},
		{"qflia_solver_proof_guard.smt2", nil, []string{"(sat-status"}},
		{"qfidl_solver_disequality_models_scopes.smt2", []string{"(diff-neq :num-exprs 0"}, []string{"(sat-status"}},
		{"qfidl_solver_disequality_pigeonhole.smt2", []string{"(diff-neq :num-exprs 1"}, []string{"(lia2pb ", "(sat-status"}},
		{"qfidl_solver_bv_sat_pipeline.smt2", []string{"(pb2bv ", "(bit-blaster ", "(sat-status"}, nil},
		{"qfidl_solver_smt_fallback.smt2", []string{"(diff-neq ", "(pb2bv ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfidl_solver_core_route.smt2", nil, []string{"(diff-neq ", "(sat-status"}},
		{"qfidl_solver_proof_route.smt2", nil, []string{"(diff-neq ", "(sat-status"}},
		{"qfidl_disequality_models_scopes.smt2", []string{"(diff-neq :num-exprs 0"}, []string{"(sat-status"}},
		{"qfidl_disequality_pigeonhole.smt2", []string{"(diff-neq :num-exprs 1"}, []string{"(lia2pb ", "(sat-status"}},
		{"qfidl_bv_sat_pipeline.smt2", []string{"(pb2bv ", "(bit-blaster ", "(sat-status"}, nil},
		{"qfidl_smt_fallback.smt2", []string{"(diff-neq ", "(pb2bv ", "(smt.tactic start)"}, []string{"(sat-status"}},
		{"qfidl_core_route.smt2", []string{"(smt.tactic start)"}, []string{"(diff-neq ", "(sat-status"}},
		{"qfidl_proof_route.smt2", []string{"(smt.tactic start)"}, []string{"(diff-neq ", "(sat-status"}},
	}
	for _, tc := range cases {
		if prefix == "qfufbv" && strings.HasPrefix(tc.name, "qfufbv_ackr_") {
			continue
		}
		if !strings.HasPrefix(tc.name, prefix+"_") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			level, err := diagnostic.ProcessVerbosityLevel()
			if err != nil {
				t.Fatal(err)
			}
			previous, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				t.Fatal(err)
			}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				for _, err := range []error{diagnostic.SetProcessVerbosityLevel(level), diagnostic.SetProcessVerboseStream(previous), stream.Close()} {
					if err != nil {
						t.Error(err)
					}
				}
			}()
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerbosityLevel(10); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err = run(bytes.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			goVerbose, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			check := func(origin, text string) {
				t.Helper()
				for _, s := range tc.require {
					if !strings.Contains(text, s) {
						t.Fatalf("%s missing route evidence %q\n%s", origin, s, text)
					}
				}
				for _, s := range tc.forbid {
					if strings.Contains(text, s) {
						t.Fatalf("%s entered unexpected stage %q\n%s", origin, s, text)
					}
				}
			}
			check("Go", goVerbose)
			if canonicalOutput(output.String()) != canonicalOutput(scriptResults[tc.name]) {
				t.Fatal("diagnostics changed the result", output.String())
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				var native, verbose bytes.Buffer
				cmd := exec.Command(reference, "-v:10", "-in")
				cmd.Stdin = bytes.NewReader(script)
				cmd.Stdout = &native
				cmd.Stderr = &verbose
				if err = cmd.Run(); err != nil {
					t.Fatal(err, native.String(), verbose.String())
				}
				check("C++", verbose.String())
				if canonicalOutput(native.String()) != canonicalOutput(output.String()) {
					t.Fatal("source result mismatch", native.String(), output.String())
				}
			}
		})
	}
}

func TestSMTLibRunnerMaxBVSharingStepFrontiers(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	for limit := 0; limit <= 40; limit++ {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			script := fmt.Sprintf(`(set-logic QF_BV)
(declare-const a (_ BitVec 8))
(declare-const b (_ BitVec 8))
(declare-const c (_ BitVec 8))
(declare-const d (_ BitVec 8))
(assert (= (bvadd a b) c))
(assert (= (bvadd b c a d) (bvadd a b c d)))
(check-sat-using (using-params max-bv-sharing :max_steps %d))
(get-info :reason-unknown)
`, limit)
			var actual bytes.Buffer
			if err := run(strings.NewReader(script), &actual); err != nil {
				t.Fatal(err, actual.String())
			}
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(script)
			native, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal(err, string(native))
			}
			if canonicalOutput(actual.String()) != canonicalOutput(string(native)) {
				t.Fatalf("Go: %s\nC++: %s", actual.String(), native)
			}
		})
	}
}

func TestSMTLibRunnerBitBlastStepFrontiers(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	for limit := 0; limit <= 60; limit++ {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			script := fmt.Sprintf(`(set-logic QF_BV)
(declare-const a (_ BitVec 3))
(declare-const b (_ BitVec 3))
(declare-const c (_ BitVec 3))
(assert (= (bvadd a b) c))
(assert (= (bvmul a c) b))
(check-sat-using (using-params bit-blast :max_steps %d))
(get-info :reason-unknown)
`, limit)
			var actual bytes.Buffer
			if err := run(strings.NewReader(script), &actual); err != nil {
				t.Fatal(err, actual.String())
			}
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(script)
			native, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal(err, string(native))
			}
			if canonicalOutput(actual.String()) != canonicalOutput(string(native)) {
				t.Fatalf("Go: %s\nC++: %s", actual.String(), native)
			}
		})
	}
}

var scriptResults = map[string]string{
	"fixedpoint_spacer_bv_state.smt2":              "unsat\nsat\nunsat\n",
	"fixedpoint_spacer_datatype_state.smt2":        "unsat\nsat\nunsat\n",
	"qfnra_nlsat_named.smt2":                       "sat\nunsat\nsat\nsat\nunsat\n",
	"unit_subsumption_named.smt2":                  "sat\nunsat\n",
	"default_named_tactic.smt2":                    "sat\nunsat\nsat\n",
	"more_tactic_factories.smt2":                   "sat\nsat\nsat\nsat\n",
	"core_new_factories.smt2":                      "sat\nsat\nsat\n",
	"fm_one_sided_mixed_integer_model.smt2":        "sat\n((x 3)\n (y (/ 1.0 2.0)))\n",
	"fm_models_and_contradiction.smt2":             "sat\n((x 3.0))\nunsat\nsat\n((x 3.0))\n",
	"fixedpoint_propositional.smt2":                "unsat\nsat\ntrue\nunsat\n",
	"fixedpoint_relation_representations.smt2":     "sat\nsat\nsat\nsat\n",
	"fixedpoint_pdr_integer.smt2":                  "sat\nunsat\n",
	"fixedpoint_pdr_background.smt2":               "sat\nunsat\n",
	"fixedpoint_pdr_recursive_trace.smt2":          "sat\n(and (P 2) (P 1) (P 0))\nunsat\n",
	"fixedpoint_pdr_nonlinear_trace.smt2":          "sat\ntrue\n",
	"fixedpoint_pdr_initial_trace.smt2":            "sat\n(and (P 2) (P 1))\n",
	"fixedpoint_pdr_arithmetic_invariant.smt2":     "unsat\n",
	"fixedpoint_pdr_convex_both.smt2":              "unsat\n",
	"fixedpoint_pdr_convex_closure.smt2":           "unsat\n",
	"fixedpoint_pdr_convex_interior.smt2":          "unsat\n",
	"fixedpoint_pdr_missing_tail.smt2":             "unsat\n",
	"fixedpoint_pdr_linear_trace.smt2":             "sat\ntrue\n",
	"fixedpoint_pdr_variable_trace.smt2":           "sat\ntrue\n",
	"fixedpoint_pdr_repeated_disequalities.smt2":   "sat\ntrue\n",
	"fixedpoint_pdr_model_generalizer.smt2":        "sat\ntrue\n",
	"fixedpoint_spacer_core.smt2":                  "sat\nunsat\n",
	"fixedpoint_spacer_acyclic_inlining.smt2":      "sat\ntrue\n",
	"fp_spacer_arith_inv.smt2":                     "unsat\n",
	"fp_spacer_linear_inv.smt2":                    "unsat\n",
	"fp_spacer_generalizers_inv.smt2":              "unsat\n",
	"fp_spacer_flexible_trace.smt2":                "unsat\n",
	"fp_spacer_skip_propagate.smt2":                "unsat\n",
	"fp_spacer_mincut_inv.smt2":                    "unsat\n",
	"fp_spacer_mutual_inv.smt2":                    "unsat\n",
	"fixedpoint_spacer_nonlinear_inlining.smt2":    "sat\ntrue\n",
	"fixedpoint_spacer_inliner_resolution.smt2":    "sat\ntrue\nunsat\n",
	"fixedpoint_spacer_unfold_rules.smt2":          "sat\n",
	"fixedpoint_spacer_coalesce_unfold.smt2":       "sat\n",
	"fixedpoint_spacer_slice_columns.smt2":         "sat\n",
	"fp_spacer_array_instantiation.smt2":           "sat\n",
	"fp_spacer_array_blast.smt2":                   "sat\n",
	"fp_spacer_bit_blast.smt2":                     "sat\n",
	"fp_spacer_scale.smt2":                         "sat\n",
	"fp_spacer_magic.smt2":                         "sat\n",
	"fp_spacer_karr.smt2":                          "unsat\n",
	"fixedpoint_spacer_recursive_trace.smt2":       "sat\n(and (P 1) (P 0))\nunsat\n",
	"horn_tactic_finite.smt2":                      "unsat\nsat\nunsat\nunsat\nsat\nunsat\nsat\nunsat\nsat\nunsat\nsat\n",
	"horn_solver_finite.smt2":                      "unsat\nsat\n",
	"fixedpoint_finite_relations.smt2":             "sat\n(and (= (:var 0) true) (= (:var 1) false))\nsat\n(= (:var 0) #b10)\nsat\nsat\n(= (:var 0) #b01)\nsat\n(= (:var 0) #b10)\nsat\nsat\nunsat\nsat\n(= (:var 0) #b10)\nsat\nsat\nunsat\nsat\nunsat\nsat\n(= (:var 0) false)\nsat\nunsat\n",
	"array_as_array_combined_fallback_gap.smt2":    "sat\nunsat\nsat\n",
	"default_strategy_fragments.smt2":              "unsat\nunsat\nunsat\nunsat\nunsat\nunsat\n",
	"qfnia_solver_bounded.smt2":                    "sat\n((x 2))\nunsat\nunsat\nunsat\n",
	"qfnia_cofactor_term_ite.smt2":                 "sat\n((p false)\n (x 2))\nunsat\n",
	"nlsat_direct.smt2":                            "sat\n((x (/ 1.0 2.0)))\nunsat\nsat\n",
	"nlsat_shuffle.smt2":                           "sat\n",
	"nlsat_inline.smt2":                            "sat\n((x 1.0)\n (y 1.0))\n",
	"qfnra_nlsat.smt2":                             "unsat\nsat\nunsat\nsat\nsat\nsat\nunsat\nsat\n",
	"qfufnra_to_int.smt2":                          "sat\nunsat\n",
	"qfufnra_integer_division.smt2":                "sat\nunsat\n",
	"qfufnra_inverse_trig.smt2":                    "unknown\nunknown\nunknown\n",
	"qfufnra_forward_trig.smt2":                    "sat\nunsat\n",
	"utvpi_mixed_sum_model.smt2":                   "sat\n",
	"nla2bv_bounded_scopes.smt2":                   "sat\n((x 7))\nunsat\nsat\n((x 7))\n",
	"nla2bv_under_unknown.smt2":                    "unknown\nsat\n(((= (* x x) 4) true))\n",
	"nla2bv_real_models.smt2":                      "sat\n(((= (* x x) 2.0) true))\nunknown\nsat\n(((= (* x x) 2.0) true))\n",
	"nla2bv_root_parameters.smt2":                  "sat\n(((= (* x x) 3.0) true))\n",
	"fd_weighted_pb.smt2":                          "sat\n((p true) (q true) (r false))\nunsat\nsat\n",
	"fd_unbounded_unknown.smt2":                    "unknown\n",
	"fd_wide_offset_model.smt2":                    "sat\n((x 36893488147419103233))\n",
	"fd_mixed_scopes_models.smt2":                  "sat\n((n 2) (colour green) (p true) (q true) (r false))\nunsat\n(too-small)\nsat\n((n 2) (colour green))\nsat\nunsat\nsat\n",
	"dt2bv_models_scopes.smt2":                     "sat\n((x green))\nunsat\n(chosen clash)\nsat\n((x green))\nsat\n((x amber))\n",
	"der_tactic.smt2":                              "unsat\n",
	"distribute_forall_tactic.smt2":                "unsat\nunsat\n",
	"ufbv_macro_tactics.smt2":                      "unsat\nunsat\n",
	"nnf_tactics.smt2":                             "unsat\nunsat\n",
	"ufbv_tactic.smt2":                             "unsat\n",
	"ufbv_solver.smt2":                             "unsat\n",
	"bv_quantified_strategy.smt2":                  "unsat\nunsat\n",
	"bv_quantified_mbqi.smt2":                      "unsat\n",
	"fpa_canonical_model_scopes.smt2":              "sat\n(((= (fp.to_ieee_bv x) #b1011001) true)\n ((fp.isNormal x) true)\n ((= rm RNA) true))\nunsat\nsat\n(((fp.isZero x) true)\n ((fp.isNegative x) true))\nsat\n(((fp.isNaN x) true)\n ((= rm RNA) true))\n",
	"fpa2bv_tactic.smt2":                           "sat\n",
	"qffp_solver_models_scopes.smt2":               "sat\n(((fp.isNaN x) true))\nunsat\nsat\nsat\n",
	"qffpbv_tactic.smt2":                           "sat\nsat\n",
	"qffp_default.smt2":                            "sat\n",
	"qffplra_solver.smt2":                          "sat\nsat\n",
	"qffplra_default.smt2":                         "sat\n",
	"nra_default_portfolio.smt2":                   "sat\nsat\nunsat\n",
	"quant_lra_lia_solver.smt2":                    "sat\nunsat\nsat\nunsat\nsat\nunsat\n",
	"qfnra_boolean.smt2":                           "sat\n",
	"user_universe_factory_scopes.smt2":            "sat\n(((distinct a b c) true)\n ((= (f a) b) true)\n ((= (f b) c) true)\n ((= (f c) a) true))\nunsat\nsat\n(((distinct a b c) true)\n ((= (f a) b) true))\n",
	"bv_native_operator_models.smt2":               "sat\n((a #xa)\n ((bvadd a b c) #xe)\n ((bvsub a b) #x7)\n ((bvmul a b c) #xe)\n ((bvudiv a b) #x3)\n ((bvurem a b) #x1)\n ((bvsdiv a b) #xe)\n ((bvsrem a b) #x0)\n ((bvsmod a b) #x0)\n ((bvshl a b) #x0)\n ((bvlshr a b) #x1)\n ((bvashr a b) #xf)\n ((ext_rotate_left a b) #x5)\n ((ext_rotate_right a b) #x5)\n ((bvumul_noovfl a b) false)\n ((bvsmul_noovfl a b) true)\n ((bvsmul_noudfl a b) false)\n ((bvredor a) #b1)\n ((bvredand a) #b0))\nunsat\nsat\nunsat\nsat\n",
	"bv_integer_conversion_scopes.smt2":            "sat\n(((= b #xf) true)\n ((= (bv2int b) 15) true)\n ((= (bv2int (bvnot b)) 0) true))\nunsat\nsat\nunsat\nsat\n",
	"bv_signed_function_model.smt2":                "sat\n(((f #x7) true)\n ((f #x5) false)\n ((= (mkbv p (not q) true false) #x7) true))\nunsat\nsat\n",
	"bv_fixed_cache_scopes.smt2":                   "sat\n(((= x #x9) true)\n ((= y #x9) true)\n ((= x y) true))\nunsat\nsat\nsat\n(((= x #x9) true)\n ((= y #xa) true)\n ((distinct x y) true)\n ((distinct (f x) (f y)) true))\nsat\n",
	"bv_private_propagation_scopes.smt2":           "sat\n((b0 true)\n (b1 true)\n (b2 true)\n (b3 true)\n (b4 true)\n ((= x #xf) true)\n ((= y #xf) true))\nunsat\nsat\nsat\n((b0 false)\n (b1 false)\n (b2 false)\n (b3 false)\n (b4 false)\n ((= x #x0) true)\n ((= y #x0) true))\nsat\n",
	"bv_bit_occurrences_scopes.smt2":               "sat\n(((= ((_ extract 0 0) x) #b1) true)\n ((= ((_ extract 2 2) x) #b1) true)\n ((= ((_ extract 3 3) x) (bvnot ((_ extract 1 1) x))) true))\nsat\n(((= x #x7) true)\n ((= y #x8) true))\nsat\n(((= x #xd) true)\n ((= y #x2) true))\nunsat\nsat\n",
	"basic_infinite_arithmetic.smt2":               "sat\n(((>= i 0) true)\n ((<= i 2) true)\n ((> r 0) true)\n ((< r (/ 1.0 3.0)) true)\n ((distinct (f r) (f (/ 1.0 2.0))) true))\nunsat\n(real_conflict)\nsat\nunsat\n(integer_conflict)\nsat\nsat\n(((= (* r r) (/ 1.0 16.0)) true))\nsat\n",
	"reset_assertions_default.smt2":                "success\nsuccess\nsuccess\nsuccess\nsuccess\nsuccess\nsuccess\nsuccess\nsat\nsuccess\nsuccess\nsat\n((x 3))\n",
	"reset_assertions_global.smt2":                 "sat\n((x 7)\n (y 5)\n ((twice x) 14))\n((x 7)\n (y 5)\n ((twice x) 14))\n((x 7))\nsat\n((x 0)\n (y 0)\n ((twice x) 0))\nsat\n((x 3)\n (y 4)\n ((twice x) 6))\nunsat\n(contradiction)\n(contradiction)\nsat\n((x 0)\n (y 0)\n ((twice x) 0))\n",
	"reset_session.smt2":                           "success\nsuccess\nsuccess\nsuccess\nsuccess\nsuccess\nsuccess\nsuccess\nsat\nsuccess\nsuccess\nsuccess\nsuccess\nsat\n(((> x 4) true))\nsuccess\nsuccess\nunsat\n(bound impossible)\nsuccess\nsat\nsuccess\nsuccess\nsuccess\nsat\nsuccess\nsuccess\nsuccess\nsat\n((answer 1))\n",
	"difference_shortcut_cores_real.smt2":          "sat\nsat\nunsat\n(full chain3 strict_close)\nsat\nunsat\n(weak_close near chain0 chain3)\nsat\n",
	"difference_shortcut_cores_int.smt2":           "sat\nsat\nunsat\n(full chain3 strict_close)\nsat\nunsat\n(weak_close near chain0 chain3)\nsat\n",
	"mbqi_trace_parameter_updates.smt2":            "sat\nsat\nsat\n",
	"mbqi_boolean_skolem_budget.smt2":              "unsat\n",
	"disabled_arithmetic_numeral_models.smt2":      "sat\n((p true))\nunknown\n(((f 3.0) (not (= 3.0 Real!val!1)))\n ((f 4.0) (not (= 4.0 Real!val!1))))\nunsat\nunknown\n(((f 3.0) (not (= 3.0 Real!val!1)))\n ((f 4.0) (not (= 4.0 Real!val!1))))\nsat\n((p true))\n",
	"integer_fractional_candidate.smt2":            "unknown\n((x 0)\n (y 0))\nunknown\n((x (- 1))\n (y (- 1)))\nsat\n((x 1)\n (y 1))\n",
	"real_interval_epsilon.smt2":                   "sat\n((x (/ 1.0 2.0)))\nsat\n((x (/ 1.0 4.0)))\nunsat\nsat\n((x (/ 1.0 4.0)))\nsat\n((x (/ 1.0 2.0)))\n",
	"incomplete_theory_candidate_model.smt2":       "unknown\n(((f a) true)\n ((f b) false)\n ((f 1.0) false))\nunsat\nunknown\n(((f a) true)\n ((f b) false)\n ((f 1.0) false))\n",
	"global_option_success_updates.smt2":           "success\nsuccess\nsuccess\nsuccess\nsuccess\nsat\nsuccess\nsat\n",
	"qfs_legacy_string_free_models.smt2":           "sat\n(((= s \"\") false))\nsat\n(((str.len s) 3))\nunsat\nsat\n((s \"abc\"))\nsat\n(((= s \"\") false))\n",
	"generic_string_z3str3_free_models.smt2":       "sat\n(((= s \"\") false))\nsat\n(((str.len s) 3))\nunsat\nsat\n((s \"abc\"))\nsat\n(((= s \"\") false))\n",
	"generic_string_auto_free_models.smt2":         "sat\n(((= s \"\") false))\nsat\n(((str.len s) 3))\nunsat\nsat\n((s \"abc\"))\nsat\n(((= s \"\") false))\n",
	"generic_auto_nonstring_sequence.smt2":         "sat\n(((= x (seq.unit 7)) true))\nunsat\nsat\n(((seq.len x) 1))\n",
	"seq_negative_symbolic_mixed.smt2":             "sat\n",
	"seq_negative_contains_canonize_scopes.smt2":   "unknown\nunsat\nsat\n(((not (str.contains x \"a\")) true))\nunsat\nsat\n(((= x \"7\") true))\nunknown\n",
	"queue_factory_fallback_scopes.smt2":           "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"queue_parameter_lifetime_scopes.smt2":         "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"theory_aware_case_split_scopes.smt2":          "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"relevance_fifo_case_split_scopes.smt2":        "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"basic_activity_case_split_scopes.smt2":        "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"delayed_activity_case_split_scopes.smt2":      "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"cached_activity_case_split_scopes.smt2":       "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"relevance_goal_case_split_scopes.smt2":        "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"relevance_activity_case_split_scopes.smt2":    "sat\n(((or a b) true)\n (c true))\nunsat\nsat\nsat\n((b true)\n (c true))\nunsat\nsat\n",
	"quant_seq_symbolic_int_prefix.smt2":           "sat\n(((= x (str.++ (int.to.str n) \"a\")) true))\nunsat\nsat\n(((= x \"7a\") true))\nsat\n",
	"quant_seq_shared_boolean_scopes.smt2":         "unsat\nsat\n(((= x z) true) ((= x y) false) ((= (f x) (f z)) true) ((= (f x) (f y)) false))\nunsat\n",
	"quant_seq_explanation_scopes.smt2":            "unsat\n(associative_words)\nunsat\nsat\n",
	"quant_seq_unit_disequality_scopes.smt2":       "unsat\n(head words tail)\nsat\nsat\nunsat\n",
	"quant_seq_residual_evidence_scopes.smt2":      "unsat\n(tails words prefixes)\nsat\nunsat\nsat\n",
	"datatype_boolean_fields_scopes.smt2":          "sat\nsat\n((p true) ((field x) true) (((_ is box) x) true))\nunsat\nsat\nsat\n((p false) ((field x) false) (((_ is box) x) true))\nunsat\n",
	"parametric_datatype_scopes.smt2":              "unsat\nsat\n",
	"parametric_sort_definitions.smt2":             "unsat\nsat\n",
	"datatype_metadata_merge_scopes.smt2":          "sat\n((((_ is right) x) true) (((_ is right) y) true) ((= x y) true))\nunsat\n(rejected_survivor)\nsat\nsat\n(((not ((_ is left) x)) true) ((not ((_ is middle) y)) true))\nunsat\n(rejected_constructor)\nsat\nsat\n(((= u v) true) ((= (rf x) u) true) ((= (rf y) v) true))\nsat\n",
	"unsat_core_reverse_trail.smt2":                "unsat\n(c)\nsat\n((a true) (b true) (c false))\nunsat\n(blocked)\nsat\nunsat\n(h)\nsat\n((e true) (h false) ((= x y) true))\nunsat\n(blocked_eq)\nsat\n",
	"closest_theory_variables.smt2":                "unsat\n(close_chain)\nsat\n(((= x0 x3) true) ((< (f x2) (f x4)) true))\nsat\n(((f x0) 7) ((f x3) 7) ((f x4) 8))\nunsat\n(bad_bound)\nsat\nsat\n",
	"pb_native_watches_scopes.smt2":                "sat\n((b true))\nsat\n((b true) (d true) (e true))\nunsat\n(rejected_watch)\nsat\nsat\nunsat\n(rejected_complement)\nsat\n(((and a b) true) ((xor c d) true))\nunsat\n(rejected_proxy)\nsat\nsat\n((W true) (e true))\nsat\n",
	"pb_strengthening_scopes.smt2":                 "unsat\n(cut_A cut_B)\nsat\nsat\n((x true) (u true) (A true) (B false))\nunsat\n(rejected_cut)\nsat\nsat\n((A true) (B false))\nsat\n",
	"pb_compilation_pigeonhole_scopes.smt2":        "unsat\nsat\n",
	"php43_max_conflicts_repeated_assuming.smt2":   "unknown\nunknown\nunknown\nunknown\nunknown\nunknown\nunknown\nunknown\nunknown\nunknown\nunknown\nunknown\n(:reason-unknown \"max-conflicts-reached\")\nunknown\nunsat\nunsat\nunsat\nunsat\nunsat\nunknown\n",
	"pb_simplex_scopes.smt2":                       "unsat\n(pair_xy pair_yz pair_xz cap_one)\nsat\nunsat\n(below_two at_least_three)\nsat\nsat\n((x true) (y true) (z false) (Parent true) (Nested true))\nsat\n",
	"datatype_cycle_scopes.smt2":                   "unsat\n(x_edge y_edge z_edge)\nsat\n(((= (left x) (right x)) true) ((= (left (left x)) leaf) true))\nunsat\nsat\nsat\n",
	"datatype_constructor_conflict_cores.smt2":     "sat\n((((_ is left) x) true) ((= (lf x) u) true))\nunsat\n(left_fact not_left)\nsat\nunsat\n(left_fact right_fact)\nsat\n",
	"datatype_incremental_recognizers.smt2":        "sat\n((((_ is aa) x) false) ((= x y) true))\nsat\n((((_ is cc) x) true) (((_ is bb) y) false))\nunsat\nsat\nsat\n",
	"datatype_argument_lazy_scopes.smt2":           "sat\n((((_ is z) x) true) (((_ is s) y) true))\nsat\n((((_ is z) x) false) (((_ is s) y) true))\nunsat\nsat\nsat\n",
	"core_validation_scopes.smt2":                  "unsat\n((not p) (not q))\nsat\nunsat\n(blocked)\nsat\nsat\n",
	"recursive_mutual_boolean_model.smt2":          "sat\n(((even 0) true) ((even 5) false) ((odd 5) true) ((choose true 7) 7) ((choose false 7) (- 7)) (zero 0))\nunsat\nsat\n",
	"recursive_mutual_safe_get_value.smt2":         "sat\n(((choose true 7) 7) ((choose false 7) (- 7)) (zero 0) ((identity true) true) ((identity false) false))\nunsat\nsat\n",
	"recursive_mutual_and_arguments.smt2":          "sat\n(((even 0) 1) ((even 5) 0) ((odd 5) 1) ((choose true 7) 7) ((choose false 7) (- 7)) (zero 0))\nunsat\nsat\n",
	"recursive_sum_model_scopes.smt2":              "sat\n(((sum 0) 0) ((sum 3) 6) ((sum 5) 15))\nunsat\nsat\n(((sum 6) 21))\n",
	"lazy_multipattern_scopes.smt2":                "unsat\nsat\n(((p c) true))\nunsat\nsat\n",
	"mbqi_restart_cores_models_scopes.smt2":        "unsat\n(blocked_true)\nsat\n(((p true) true) ((p false) true))\nunsat\n(blocked_false)\nsat\n(((p true) true) ((p false) true))\n",
	"check_base_conflict_repeated.smt2":            "unsat\nunsat\n()\nsat\nunsat\n(p)\nsat\n",
	"setup_check_tactic_model_scopes.smt2":         "sat\n((x 7))\nunsat\nsat\n((x 7))\n",
	"quantifier_theory_roundtrip.smt2":             "sat\n(((p true) true) ((p false) true) ((select a 0) 7))\nunsat\nsat\n(((p true) true) ((p false) true) ((select a 0) 7))\n",
	"asserted_substitution_scopes.smt2":            "sat\n((x 7))\nunsat\nsat\n((x 7))\nsat\n((x 8))\n",
	"assertion_conflict_prefix_scopes.smt2":        "unsat\nunsat\nunsat\nsat\n",
	"asserted_driver_inconsistency_scopes.smt2":    "unsat\nsat\nunsat\nsat\n",
	"asserted_driver_quantifier_scopes.smt2":       "sat\nunsat\nsat\n",
	"quant_seq_boundary_equality_core.smt2":        "unsat\n(same words)\n",
	"quant_seq_boundary_length_core.smt2":          "unsat\n(length words)\n",
	"quant_seq_unit_character_core.smt2":           "unsat\n(character different)\n",
	"quant_seq_empty_backjump.smt2":                "unsat\n",
	"quant_seq_regex_backjump.smt2":                "unsat\n",
	"quant_seq_bounded_regex_backjump.smt2":        "unsat\n",
	"quant_auflia_core.smt2":                       "unsat\n(axiom negative)\nunsat\n(axiom negative)\nunsat\n(axiom negative)\n",
	"quant_ufnia_core.smt2":                        "unsat\n(axiom negative)\nunsat\n(axiom negative)\nunsat\n(axiom negative)\n",
	"quant_ufnia_models_scopes.smt2":               "sat\n(((>= x 9) true))\nsat\n(((>= x 9) true))\nunsat\nsat\n(((>= x 9) true))\n",
	"quant_uflra_proof.smt2":                       "unsat\nunsat\n",
	"quant_uflra_models_scopes.smt2":               "sat\n(((>= x 3) true))\nsat\n(((>= x 3) true))\nunsat\nsat\n(((>= x 3) true))\n",
	"quant_auflia_models_scopes.smt2":              "sat\n(((>= x 3) true))\nsat\n(((>= x 3) true))\nunsat\nsat\n(((>= x 3) true))\n",
	"quant_auflira_models_scopes.smt2":             "sat\n(((>= x 3) true) ((= i 4) true))\nsat\n(((>= x 3) true) ((= i 4) true))\nunsat\nsat\n(((>= x 3) true) ((= i 4) true))\n",
	"quant_aufnira_models_scopes.smt2":             "sat\n(((>= x 9) true))\nsat\n(((>= x 9) true))\nunsat\nsat\n(((>= x 9) true))\n",
	"qfufbv_ackr_proof_refusal.smt2":               "unknown\n(:reason-unknown \"qfufbv_ackr does not support proof production\")\nsat\n",
	"qfufbv_ackr_core_refusal.smt2":                "unknown\n(:reason-unknown \"qfufbv_ackr does not support unsat core production\")\nsat\n",
	"qfufbv_ackr_smt_fallback.smt2":                "sat\n((x 1))\n",
	"qfufbv_ackr_smt_eager.smt2":                   "sat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\n",
	"qfufbv_ackr_smt_lazy.smt2":                    "sat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\n",
	"qfufbv_ackr_sat_eager.smt2":                   "sat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\n",
	"qfufbv_ackr_sat_lazy.smt2":                    "sat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\n",
	"qfufbv_ackr_incsat_eager.smt2":                "sat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\n",
	"qfufbv_ackr_incsat_lazy.smt2":                 "sat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true) ((bvult (f x) (f y)) true))\n",
	"qfufbv_solver_assumptions.smt2":               "unsat\n(p)\nsat\n((p false))\nunsat\nsat\n((p false))\n",
	"qfufbv_core.smt2":                             "unsat\n(impossible)\n",
	"qfufbv_limits_recovery.smt2":                  "unknown\n(:reason-unknown \"max. steps exceeded\")\nsat\n(((bvult x y) true)\n ((bvult (f x) (f y)) true))\n",
	"qfufbv_proof.smt2":                            "unsat\n",
	"qfufbv_reduce_models_scopes.smt2":             "sat\n(((bvult x y) true)\n ((bvult (f #x1 x) (f #x1 y)) true))\nunsat\nsat\n(((bvult x y) true)\n ((bvult (f #x1 x) (f #x1 y)) true))\n",
	"qfufbv_sat_models_scopes.smt2":                "sat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true)\n ((= (f #x0) #x1) true))\nunsat\nsat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true)\n ((= (f #x0) #x1) true))\n",
	"qfufbv_smt_models_scopes.smt2":                "sat\n(((bvult x y) true)\n ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true)\n ((bvult (f x) (f y)) true))\n",
	"qfufbv_solver_core.smt2":                      "unsat\n(impossible)\n",
	"qfufbv_solver_proof.smt2":                     "unsat\n",
	"qfufbv_solver_reduce_models_scopes.smt2":      "sat\n(((bvult x y) true)\n ((bvult (f #x1 x) (f #x1 y)) true))\nunsat\nsat\n(((bvult x y) true)\n ((bvult (f #x1 x) (f #x1 y)) true))\n",
	"qfufbv_solver_sat_models_scopes.smt2":         "sat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true)\n ((= (f #x0) #x1) true))\nunsat\nsat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true)\n ((= (f #x0) #x1) true))\n",
	"qfufbv_solver_smt_models_scopes.smt2":         "sat\n(((bvult x y) true)\n ((bvult (f x) (f y)) true))\nunsat\nsat\n(((bvult x y) true)\n ((bvult (f x) (f y)) true))\n",
	"bv_bound_core_refusal.smt2":                   "unknown\n(:reason-unknown \"bv-bound-chk does not support unsat core production\")\nunsat\n(yes no)\n",
	"bv_bound_proof_refusal.smt2":                  "unknown\n(:reason-unknown \"bv-bound-chk does not support proof production\")\nunsat\n",
	"bv_bound_models_scopes.smt2":                  "sat\n(((= x #x02) true))\nunsat\nsat\n(((= x #x02) true))\n",
	"bv_bound_inconsistent.smt2":                   "unsat\n",
	"card2bv_models_scopes.smt2":                   "sat\n((((_ at-most 2) p q r s v) true)\n (((_ at-least 2) p q r s v) true))\nunsat\nsat\n((((_ at-most 2) p q r s v) true)\n (((_ at-least 2) p q r s v) true))\n",
	"card2bv_core.smt2":                            "unsat\n(limit)\n",
	"card2bv_weighted_models.smt2":                 "sat\n((((_ pbeq 3 1 2 3) p q r) true))\nunsat\nsat\n((((_ pbeq 3 1 2 3) p q r) true))\n",
	"reduce_args_common_ground_model.smt2":         "sat\n(((f a x) true)\n ((f a y) false)\n ((distinct x y) true))\n",
	"reduce_args_core_refusal.smt2":                "unknown\n(:reason-unknown \"reduce-args does not support unsat core production\")\nunsat\n(yes no)\n",
	"reduce_args_mixed_sort_model.smt2":            "sat\n(((f 1 p x) #xa)\n ((f 2 q y) #xb))\n",
	"reduce_args_nested_model.smt2":                "sat\n(((f (g #x1) x) true)\n ((f (g #x1) y) false)\n ((distinct x y) true))\n",
	"reduce_args_offset_model.smt2":                "sat\n(((f (bvadd #x1 x) y) true)\n ((f (bvadd #x2 x) z) false))\n",
	"reduce_args_proof_refusal.smt2":               "unknown\n(:reason-unknown \"reduce-args does not support proof production\")\nunsat\n",
	"reduce_args_values_models_scopes.smt2":        "sat\n(((f #x0 x) #x11)\n ((f #x1 y) #x22))\nunsat\nsat\n(((f #x0 x) #x11)\n ((f #x1 y) #x22))\n",
	"qfaufbv_core.smt2":                            "unsat\n(impossible)\n",
	"qfaufbv_extensional_models.smt2":              "sat\n(((distinct a b) true)\n ((= (select a #b00) (select b #b00)) true))\n",
	"qfaufbv_limits_recovery.smt2":                 "unknown\n(:reason-unknown \"max. steps exceeded\")\nsat\n(((= (bvmul (select a i) (select a i)) #x9) true))\n",
	"qfaufbv_models_scopes.smt2":                   "sat\n(((= (select a i) #x23) true)\n ((= (select a j) #x45) true)\n ((bvult (f i) (f j)) true)\n ((bvult i j) true))\nunsat\nsat\n(((= (select a i) #x23) true)\n ((= (select a j) #x45) true)\n ((bvult (f i) (f j)) true))\n",
	"qfaufbv_proof.smt2":                           "unsat\n",
	"qfaufbv_solver_core.smt2":                     "unsat\n(impossible)\n",
	"qfaufbv_solver_extensional_models.smt2":       "sat\n(((distinct a b) true)\n ((= (select a #b00) (select b #b00)) true))\n",
	"qfaufbv_solver_models_scopes.smt2":            "sat\n(((= (select a i) #x23) true)\n ((= (select a j) #x45) true)\n ((bvult (f i) (f j)) true)\n ((bvult i j) true))\nunsat\nsat\n(((= (select a i) #x23) true)\n ((= (select a j) #x45) true)\n ((bvult (f i) (f j)) true))\n",
	"qfaufbv_solver_proof.smt2":                    "unsat\n",
	"qfaufbv_store_sort_models.smt2":               "sat\n(((= (select b i) #x01) true)\n ((= (select b j) #x02) true)\n ((bvult i j) true))\n",
	"qfbv_limits_recovery.smt2":                    "unknown\n(:reason-unknown \"max. steps exceeded\")\nsat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true))\n",
	"qfbv_core.smt2":                               "unsat\n(impossible)\n",
	"qfbv_div0_fallback.smt2":                      "sat\n(((bvult (bvudiv x #x0) (bvudiv y #x0)) false)\n ((distinct x y) true))\n",
	"qfbv_incremental_sat_backend.smt2":            "p cnf 3 1\n1 2 3 0\nc 1 p\nc 2 q\nsat\n",
	"qfbv_equality_models.smt2":                    "sat\n(((not (= ((_ extract 1 0) x) ((_ extract 1 0) y))) true))\n",
	"qfbv_proof.smt2":                              "unsat\n",
	"qfbv_sat_models_scopes.smt2":                  "sat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true))\nunsat\nsat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true))\n",
	"qfbv_solver_core.smt2":                        "unsat\n(impossible)\n",
	"qfbv_solver_equality_models.smt2":             "sat\n(((not (= ((_ extract 1 0) x) ((_ extract 1 0) y))) true))\n",
	"qfbv_solver_proof.smt2":                       "unsat\n",
	"qfbv_solver_sat_models_scopes.smt2":           "sat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true))\nunsat\nsat\n(((= (bvmul x y) #x6) true)\n ((bvult x y) true))\n",
	"qfbv_solver_signed_model.smt2":                "sat\n(((bvsle #xf8 x) true)\n ((bvsle x #xff) true)\n ((not (= x #xff)) true))\n",
	"qfbv_uf_models.smt2":                          "sat\n(((distinct x y) true)\n ((bvult (f x) (f y)) true))\n",
	"ackermann_models_scopes.smt2":                 "sat\n((x #x1)\n (y #x2)\n ((f x) #x3)\n ((f y) #x4))\nunsat\nsat\n(((f x) #x3)\n ((f y) #x4))\n",
	"ackermann_nested_model.smt2":                  "sat\n(((g x) #x2)\n ((f (g x)) #x7)\n ((f y) #x7)\n ((= (f (g x)) (f y)) true))\n",
	"ackermann_nested_unsat.smt2":                  "unsat\n",
	"ackermann_boolean_models.smt2":                "sat\n(((f true) true)\n ((f false) false))\n",
	"ackermann_usort_models.smt2":                  "sat\n(((distinct x y) true)\n ((= (f x) y) true)\n ((= (f y) x) true))\n",
	"ackermann_limit_fallback.smt2":                "unknown\n(:reason-unknown \"incomplete\")\nsat\n(((f #x0) #x1)\n ((f #x1) #x2)\n ((f #x2) #x3))\n",
	"ackermann_proof_refusal.smt2":                 "unknown\n(:reason-unknown \"ackermannize does not support proof production\")\nunsat\n",
	"ackermann_core_refusal.smt2":                  "unknown\n(:reason-unknown \"ackermannize does not support unsat core production\")\nunsat\n(eq conflict)\n",
	"ackermann_div0_congruence.smt2":               "unsat\n",
	"bv_size_signed_models_scopes.smt2":            "sat\n((x #xff))\nunsat\nsat\n((x #xff))\n",
	"bv_size_negative_model.smt2":                  "sat\n((x #xff)\n ((bvsle #xf8 x) true)\n ((bvsle x #xff) true))\n",
	"bv_size_positive_model.smt2":                  "sat\n((x #x03))\n",
	"bv_size_singleton_model.smt2":                 "sat\n((x #xfb))\n",
	"bv_size_unsigned_unchanged.smt2":              "unknown\n(:reason-unknown \"incomplete\")\nsat\n((x #x02))\n",
	"bv_size_proof_refusal.smt2":                   "unknown\n(:reason-unknown \"bv-size-reduction does not support proof production\")\nunsat\n",
	"bv_size_core_refusal.smt2":                    "unknown\n(:reason-unknown \"bv-size-reduction does not support unsat core production\")\nunsat\n(lo hi)\n",
	"bv1_blast_models_scopes.smt2":                 "sat\n((x #xa5))\nunsat\nsat\n((x #xa5))\n",
	"bv1_blast_uf_models.smt2":                     "sat\n((x #b101)\n ((f x) #b101)\n ((= (f x) x) true))\n",
	"bv1_blast_core_route.smt2":                    "unsat\n(five conflict)\n",
	"bv1_blast_proof_route.smt2":                   "unsat\n",
	"bv1_blast_refusal_fallback.smt2":              "unknown\n(:reason-unknown \"bv1 blaster cannot be applied to goal\")\nsat\n(((= (bvadd x x) #b0110) true))\n",
	"bv1_blast_step_memory_limits.smt2":            "(error \"tactic failed: max. steps exceeded\")\n(:reason-unknown \"max. steps exceeded\")\nunknown\n(:reason-unknown \"max. memory exceeded\")\nsat\n((x #xa))\n",
	"qflia_solver_pb_sat_pipeline.smt2":            "sat\n((a 1)\n (b 0)\n (c 1)\n (d 1)\n (e 0)\n (f 1)\n (g 0)\n (h 1)\n (i 0))\nunsat\nsat\n((a 1)\n (b 0)\n (c 1)\n (d 1)\n (e 0)\n (f 1)\n (g 0)\n (h 1)\n (i 0))\n",
	"qflia_solver_unbounded_ilp.smt2":              "sat\n(((>= (+ x y) 1) true)\n ((<= (+ (* 2 x) y) 3) true)\n ((>= (+ x (* 2 y)) 0) true))\n",
	"qflia_solver_pseudo_Boolean.smt2":             "sat\n(((>= (+ (* 2 x) (* 3 y) (* 5 z)) 7) true)\n ((not (= x y)) true))\n",
	"qflia_solver_quasi_pb.smt2":                   "sat\n((x 90)\n (y 1))\n",
	"qflia_solver_bounded.smt2":                    "sat\n(((not (= x y)) true)\n ((or (>= (+ x y) 219) (<= (+ x y) 201)) true))\n",
	"qflia_solver_unbounded_disjunction.smt2":      "sat\n(((not (= x y)) true)\n ((>= (+ (* 2 x) y) 2) true))\n",
	"qflia_solver_proof_guard.smt2":                "unsat\n",
	"qflia_solver_core_guard.smt2":                 "unsat\n",
	"qflia_solver_models_scopes.smt2":              "sat\n((x 7))\nunsat\nsat\n((x 7))\n",
	"add_bounds_default_models_scopes.smt2":        "sat\n((x 1))\nunknown\n(:reason-unknown \"under-approximated goal found to be unsat\")\nsat\n((x 1))\n",
	"add_bounds_existing_bounds.smt2":              "unknown\n(:reason-unknown \"under-approximated goal found to be unsat\")\nsat\n(((and (>= x 10) (<= x 20)) true))\n",
	"add_bounds_negative_params.smt2":              "sat\n(((and (>= x (- 16)) (<= x (- 1))) true))\n",
	"add_bounds_fractional_real.smt2":              "sat\n(((and (>= x 0.5) (<= x 1.5)) true))\n",
	"add_bounds_boolean_precise.smt2":              "sat\nunsat\n",
	"add_bounds_proof_under.smt2":                  "unknown\n(:reason-unknown \"under-approximated goal found to be unsat\")\n",
	"ctx_simplify_memory_refusal.smt2":             "unknown\n(:reason-unknown \"max. memory exceeded\")\nsat\nsat\n",
	"ctx_solver_simplify.smt2":                     "sat\n",
	"ctx_simplify_forward_backward.smt2":           "sat\n",
	"ctx_simplify_ite.smt2":                        "sat\n",
	"ctx_simplify_or.smt2":                         "sat\n",
	"ctx_simplify_cache.smt2":                      "sat\n",
	"ctx_simplify_cores.smt2":                      "unsat\nunsat\n(a b)\n",
	"ctx_simplify_proofs.smt2":                     "unsat\nunsat\n",
	"ctx_simplify_limits.smt2":                     "sat\nsat\nsat\nsat\nsat\n",
	"ctx_simplify_models_scopes.smt2":              "sat\n((x 3)\n (y 4))\nunsat\nsat\n((x 3)\n (y 4))\n",
	"qfidl_solver_bv_sat_pipeline.smt2":            "sat\n((a 0)\n (b 1)\n (c 1))\nunsat\nsat\n((a 0)\n (b 1)\n (c 1))\n",
	"qfidl_solver_core_route.smt2":                 "unsat\n(lo hi)\n",
	"qfidl_solver_disequality_models_scopes.smt2":  "sat\n((x 0)\n (y 1)\n (z 2))\nunsat\nsat\n((x 0)\n (y 1)\n (z 2))\n",
	"qfidl_solver_disequality_pigeonhole.smt2":     "unsat\n",
	"qfidl_solver_proof_route.smt2":                "unsat\n",
	"qfidl_solver_smt_fallback.smt2":               "sat\n(((and (= (- x y) 37) (>= x 0) (<= x 100) (>= y 0) (<= y 100)) true))\nunsat\nsat\n(((= (- x y) 37) true))\n",
	"qfidl_bv_sat_pipeline.smt2":                   "sat\n((a 1)\n (b 0)\n (c 1)\n (d 1)\n (e 0)\n (f 1)\n (g 0)\n (h 1)\n (i 0))\nunsat\nsat\n((a 1)\n (b 0)\n (c 1)\n (d 1)\n (e 0)\n (f 1)\n (g 0)\n (h 1)\n (i 0))\n",
	"qfidl_core_route.smt2":                        "unsat\n(lo hi)\n",
	"qfidl_disequality_models_scopes.smt2":         "sat\n((x 0)\n (y 1)\n (z 2))\nunsat\nsat\n((x 0)\n (y 1)\n (z 2))\n",
	"qfidl_disequality_pigeonhole.smt2":            "unsat\n",
	"qfidl_limits_fallback.smt2":                   "unknown\n(:reason-unknown \"max. steps exceeded\")\nsat\n((x 7))\n",
	"qfidl_parameter_scopes.smt2":                  "sat\n((x 7))\n",
	"qfidl_proof_route.smt2":                       "unsat\n",
	"qfidl_smt_fallback.smt2":                      "sat\n(((and (= (- x y) 37) (>= x 0) (<= x 100) (>= y 0) (<= y 100)) true))\nunsat\nsat\n(((= (- x y) 37) true))\n",
	"sat_global_options.smt2":                      "unknown\n(:reason-unknown \"incomplete\")\nsat\n((p false)\n (q true))\nsat\n((p false)\n (q true))\np cnf 2 2\n-1 0\n1 2 0\nc 0 p\nc 1 q\nunknown\n(:reason-unknown \"incomplete\")\nsat\n((p false)\n (q true))\nsat\n((p false)\n (q true))\n",
	"sat_boolean_models_scopes.smt2":               "sat\n((p false)\n (q true)\n (r false))\nunsat\nsat\n((p false)\n (q true)\n (r false))\n",
	"sat_cores.smt2":                               "unsat\n(choice nq np)\n",
	"sat_dimacs.smt2":                              "p cnf 4 5\n-1 3 0\n-2 3 0\n-3 4 0\n3 -4 0\n1 2 -3 0\nc 0 p\nc 1 q\nc 3 r\nunknown\n(:reason-unknown \"incomplete\")\n",
	"sat_interpreted_residual.smt2":                "unknown\n(:reason-unknown \"incomplete\")\nsat\n((x 2)\n (p false))\nunsat\nsat\n((x 2)\n (p false))\n",
	"sat_limit_recovery.smt2":                      "unknown\n(:reason-unknown \"incomplete\")\nsat\n((p false)\n (q true))\n",
	"sat_pb_bv_pipeline.smt2":                      "sat\n((x 1)\n (y 1)\n (z 1)\n (w 1))\nunsat\nsat\n((x 1)\n (y 1)\n (z 1)\n (w 1))\n",
	"sat_preprocess_pipeline.smt2":                 "sat\n((p true)\n (q false)\n (r true))\n",
	"sat_unsupported_guard.smt2":                   "unknown\n(:reason-unknown \"operator (xor p q) not supported, apply simplifier before invoking translator\")\nsat\n((p true)\n (q false))\n",
	"aig_models_scopes.smt2":                       "sat\n((p true) (q false) (r false))\nunsat\nsat\n((p true) (q false) (r false))\n",
	"aig_core_modes.smt2":                          "unknown\n(:reason-unknown \"aig does not support unsat core production\")\nunsat\n(nq choice np)\n",
	"aig_proof_guard.smt2":                         "unknown\n(:reason-unknown \"aig does not support proof production\")\nunsat\n",
	"aig_memory_recovery.smt2":                     "unknown\n(:reason-unknown \"max. memory exceeded\")\nsat\n(((or p q) true))\n",
	"aig_bit_blast_pipeline.smt2":                  "sat\n((x 1) (y 1) (z 1) (w 1))\n",
	"bit_blast_division_refusal.smt2":              "unknown\n(:reason-unknown \"operator is not supported, you must simplify the goal before applying bit-blasting\")\nsat\n((x #x6) (y #x3))\n",
	"bit_blast_disabled_arithmetic.smt2":           "sat\n((x #x1) (y #x2))\n",
	"bit_blast_full_uf.smt2":                       "sat\n((x #x2) ((= (f x) #x5) true))\n",
	"bit_blast_models_scopes.smt2":                 "sat\n((x #x0d) (y #x03) ((bvudiv x y) #x04) ((bvurem x y) #x01))\nunsat\nsat\n((x #x0d) (y #x03))\n",
	"bit_blast_cores.smt2":                         "unsat\n(yvalue sum xvalue)\n",
	"bit_blast_limits.smt2":                        "unknown\n(:reason-unknown \"max. steps exceeded\")\nsat\n((x #x2))\n",
	"bit_blast_proof_quant_guard.smt2":             "unknown\n(:reason-unknown \"quantified variable blasting does not support proof generation\")\nunsat\nunsat\n",
	"bit_blast_quantified.smt2":                    "sat\n",
	"bit_blast_pb_pipeline.smt2":                   "sat\n((x 1) (y 1) (z 1) (w 1))\n",
	"max_bv_sharing_atomic_limit.smt2":             "sat\n((p true))\n",
	"max_bv_sharing_model_scopes.smt2":             "sat\n((x #x01) (y #x02) (z #x03) (w #x04))\nunsat\nsat\n((x #x01) (y #x02) (z #x03) (w #x04))\n",
	"max_bv_sharing_limit_recovery.smt2":           "(error \"tactic failed: max. steps exceeded\")\n(:reason-unknown \"max. steps exceeded\")\n(error \"tactic failed: max. steps exceeded\")\n(:reason-unknown \"max. steps exceeded\")\nsat\n((p false))\n",
	"max_bv_sharing_proofs.smt2":                   "unsat\n",
	"max_bv_sharing_cores.smt2":                    "unsat\n(zero one)\n",
	"max_bv_sharing_pb_pipeline.smt2":              "sat\n((x 1) (y 1) (z 1) (w 1))\n",
	"propagate_ineqs_model_scopes.smt2":            "sat\n((x 2.0) (y 5.0))\nunsat\nsat\n((x 2.0) (y 5.0))\n",
	"propagate_ineqs_core_fallback.smt2":           "unknown\n(:reason-unknown \"propagate-ineqs does not support unsat core production\")\nunsat\n(lo hi)\n",
	"propagate_ineqs_pb_pipeline.smt2":             "sat\n((x 7) (y 7))\n",
	"propagate_ineqs_proof_fallback.smt2":          "unknown\n(:reason-unknown \"propagate-ineqs does not support proof production\")\nunsat\n",
	"pb2bv_equal_vector_model.smt2":                "sat\n((x 1) (y 1) (z 0) (w 0))\n",
	"pb2bv_refusal_fallback.smt2":                  "unknown\n(:reason-unknown \"goal is in a fragment unsupported by pb2bv. Offending expression: x\")\nsat\n(((> x 0) true))\n",
	"pb2bv_proof_fallback.smt2":                    "unsat\n",
	"pb2bv_clauses_model_scopes.smt2":              "sat\n((x 1)\n (y 1)\n (z 1))\nunsat\nsat\n((x 1)\n (y 1)\n (z 1))\n",
	"pb2bv_cardinality_model.smt2":                 "sat\n(((= (+ x y z w v) 3) true))\n",
	"pb2bv_bitvector_model.smt2":                   "sat\n((x 1)\n (y 1)\n (z 1)\n (w 1))\n",
	"pb2bv_split_model.smt2":                       "sat\n((x 0)\n (y 0)\n (z 1)\n (w 1))\n",
	"pb2bv_nested_model.smt2":                      "sat\n((x 1)\n (y 1)\n (z 1))\n",
	"pb2bv_shifted_lia_pipeline.smt2":              "sat\n((x 2)\n (y 7))\n",
	"pb2bv_core.smt2":                              "unsat\n(total yhi ylo xlo xhi)\n",
	"bv_weighted_ite_model.smt2":                   "sat\n((a true)\n (b true)\n (c true)\n (d true))\n",
	"arith_negated_weighted_branches.smt2":         "sat\n(((and (>= (+ a (* 2 b) (* 4 c)) 6) (<= (+ a (* 2 b) (* 4 c)) 7)) true))\nunsat\nsat\n(((and (>= (+ a (* 2 b) (* 4 c)) 6) (<= (+ a (* 2 b) (* 4 c)) 7)) true))\n",
	"lia2pb_model_scopes.smt2":                     "sat\n((x 5) (y 2))\nunsat\nsat\n((x 5) (y 2))\n",
	"lia2pb_core.smt2":                             "unsat\n(total yhi ylo xlo xhi)\n",
	"lia2pb_partial.smt2":                          "unknown\n(:reason-unknown \"goal is in a fragment unsupported by lia2pb\")\nsat\n((x 3) (y 2))\n",
	"lia2pb_budget_recovery.smt2":                  "unknown\n(:reason-unknown \"lia2pb failed, number of necessary bits exceeds specified threshold (use option :lia2pb-total-bits to increase threshold)\")\nsat\n(((and (>= x 6) (<= x 7)) true))\n",
	"lia2pb_proof_fallback.smt2":                   "unsat\n",
	"lia2pb_shifted_pipeline.smt2":                 "sat\n((x 2) (y 7))\n",
	"diff_neq_backtrack_model.smt2":                "sat\n((x 1) (y 0))\nsat\n((x 1) (y 0))\n",
	"diff_neq_core_fallback.smt2":                  "unsat\n(lower upper)\n",
	"diff_neq_dl_pipeline_scopes.smt2":             "sat\n((x 0) (y 0) (z 1))\nunsat\nsat\n((x 0) (y 0) (z 1))\n",
	"diff_neq_parameter_recovery.smt2":             "unknown\n(:reason-unknown \"goal is not diff neq\")\nsat\n((x 0))\n",
	"diff_neq_proof_fallback.smt2":                 "unsat\n",
	"diff_neq_real_refusal.smt2":                   "unknown\n(:reason-unknown \"goal is not diff neq\")\nsat\n(((and (>= x 0) (<= x 1)) true))\n",
	"diff_neq_refusal_fallback.smt2":               "unknown\n(:reason-unknown \"goal is not diff neq\")\nsat\n((x 1))\n",
	"diff_neq_shifted_model.smt2":                  "sat\n((x (- 2)) (y 5))\n",
	"diff_neq_signed_offsets.smt2":                 "sat\n((x 0) (y 2))\n",
	"diff_neq_triangle_unsat.smt2":                 "unsat\n",
	"normalize_bounds_core.smt2":                   "unsat\n(lower upper)\n",
	"normalize_bounds_disjunction.smt2":            "sat\n((x 2))\n",
	"normalize_bounds_fix_dl_pipeline.smt2":        "sat\n((x 0) ((<= (- x y) 2) true) ((<= (- x z) 3) true) ((<= (- x w) 4) true))\nunsat\nsat\n",
	"normalize_bounds_int_model_scopes.smt2":       "sat\n(model (define-fun x () Int 7)\n)\nunsat\nsat\n((x 7))\n",
	"normalize_bounds_negative_model.smt2":         "sat\n(model (define-fun x () Int (- 3))\n)\n",
	"normalize_bounds_proof.smt2":                  "unsat\n",
	"normalize_bounds_real_default.smt2":           "sat\n((x (/ 2.0 3.0)))\n",
	"normalize_bounds_real_opt_in.smt2":            "sat\n(((and (> x (/ 1 3)) (<= x (/ 2 3))) true))\nunsat\nsat\n",
	"fix_dl_absolute_refusal.smt2":                 "sat\n((x 5) ((<= (- x y) 2) true))\n",
	"fix_dl_core.smt2":                             "unsat\n(forward backward)\n",
	"fix_dl_foreign_offset.smt2":                   "sat\n((x 0) ((p (+ 2 x)) true))\n",
	"fix_dl_model_scopes.smt2":                     "sat\n((x 0) ((<= (- x y) 2) true))\nunsat\nsat\n((x 0))\n",
	"fix_dl_parameter_recovery.smt2":               "unknown\nsat\n((x 0))\n",
	"fix_dl_proof.smt2":                            "unsat\n",
	"fix_dl_real_models.smt2":                      "sat\n((x 0.0) ((< (- x y) (/ 1 3)) true) ((<= (- x z) (- (/ 1 7))) true))\n",
	"symmetry_ranges_scopes.smt2":                  "sat\n(((distinct x y) true))\nunsat\nsat\n",
	"symmetry_function_support.smt2":               "sat\n(((distinct (f a) (f b)) true))\nunsat\nsat\n",
	"symmetry_asymmetric.smt2":                     "sat\n(((R a b) true) ((R b a) false))\n",
	"symmetry_proof_guard.smt2":                    "unsat\n",
	"qfuf_ranges_scopes.smt2":                      "sat\n(((distinct x y) true))\nunsat\nsat\n",
	"qfuf_combined_ranges_scopes.smt2":             "sat\n(((distinct x y) true))\nunsat\nsat\n",
	"qfuf_core.smt2":                               "unsat\n(different equal)\n",
	"qfuf_combined_core.smt2":                      "unsat\n(equal different)\n",
	"qfuf_proof.smt2":                              "unsat\n",
	"qfuf_parameter_recovery.smt2":                 "unknown\nsat\n(((distinct (f a) (f b)) true))\nsat\n",
	"qflra_combined_assumptions.smt2":              "sat\nsat\nunsat\nsat\nunsat\nsat\n",
	"qflra_combined_core.smt2":                     "unsat\n(positive nonpositive)\n",
	"qflra_core.smt2":                              "unsat\n(positive nonpositive)\n",
	"qflra_proof.smt2":                             "unsat\n",
	"qflra_combined_model_scopes.smt2":             "sat\n(((and (> x 0) (< x (/ 1 3)) (= y (+ x (/ 1 7)))) true))\nunsat\nsat\n",
	"qflra_model_scopes.smt2":                      "sat\n(((and (> x 0) (< x (/ 1 3)) (= y (+ x (/ 1 7)))) true))\nunsat\nsat\n",
	"qflra_parameter_overrides.smt2":               "sat\nsat\n(((and (>= (+ x y) 1) (>= (+ x (* 2 y)) 3)) true))\nsat\n",
	"qfauflia_combined_model_scopes.smt2":          "sat\n((x 2) ((= (select a y) 9) true) ((and (>= y 2) (<= y 4)) true))\nunsat\nsat\n((x 2) ((= (select a y) 9) true) ((and (>= y 2) (<= y 4)) true))\n",
	"qfauflia_combined_assumptions.smt2":           "sat\nsat\nunsat\nsat\nunsat\nsat\n",
	"qfauflia_combined_core.smt2":                  "unsat\n(zero lower)\n",
	"qfauflia_combined_proof.smt2":                 "unsat\n",
	"qfauflia_model_scopes.smt2":                   "sat\n((x 2) ((= (select a y) 9) true) ((and (>= y 2) (<= y 4)) true))\nunsat\nsat\n((x 2) ((= (select a y) 9) true) ((and (>= y 2) (<= y 4)) true))\n",
	"qfauflia_as_array_scopes.smt2":                "sat\nunsat\nsat\n",
	"qfauflia_core.smt2":                           "unsat\n(lower zero)\n",
	"qfauflia_proof.smt2":                          "unsat\n",
	"qfauflia_parameter_recovery.smt2":             "unknown\nsat\n(((and (>= x 0) (<= x 3)) true))\nsat\n(((and (>= x 0) (<= x 3)) true))\n",
	"elim_uncnstr_signed_bounds.smt2":              "sat\n(((bvsle a #x8) true) ((bvsle #x7 b) true) ((bvule c #xf) true) ((bvule #xf d) true) ((<= 4 i) true) ((>= 8 j) true))\n",
	"elim_uncnstr_constructor_singleton.smt2":      "sat\n(((f (box i)) true))\nunsat\nsat\n",
	"elim_uncnstr_uninterpreted_guard.smt2":        "unsat\n",
	"elim_uncnstr_ite_models.smt2":                 "sat\n(((= (ite c1 x y) 4) true) ((= (ite c2 z 2) 5) true) ((= (ite c3 2 w) 6) true))\n",
	"elim_uncnstr_bv_division_models.smt2":         "sat\n(((= (bvudiv a b) #x17) true) ((= (bvsdiv c d) #xfd) true) ((= (bvmul (_ bv3 128) x) (_ bv12 128)) true) ((= (bvmul #x02 y) #x06) true))\n",
	"elim_uncnstr_datatype_scopes.smt2":            "sat\nsat\n(((is-on f) true))\nsat\n",
	"elim_uncnstr_boolean.smt2":                    "sat\n(((or (not a) (= b c)) true))\n",
	"elim_uncnstr_arithmetic.smt2":                 "sat\n(((= (+ (* 2.0 x) y) 17.0) true) (i 3) ((<= j 4) true) ((>= k 8) true))\n",
	"elim_uncnstr_bitvectors.smt2":                 "sat\n(((= (bvmul #x03 x) #x0c) true) ((= ((_ extract 5 2) y) #xa) true) ((= (bvnot z) #x12) true) ((= (concat a b) #xab) true) ((= (bvor c d) #xf) true) ((bvule e #x0) true))\n",
	"elim_uncnstr_arrays.smt2":                     "sat\n(((select a true) 9) ((= (store b false v) ((as const (Array Bool Int)) 7)) true))\n",
	"elim_uncnstr_datatypes.smt2":                  "sat\n(((val b) 7) ((not (= n zero)) true))\n",
	"elim_uncnstr_core_scopes.smt2":                "sat\n(((not a) true) ((or a b) true))\nunsat\n(other negative disjunction)\nsat\n(((not a) true) ((or a b) true))\n",
	"elim_uncnstr_proofs.smt2":                     "sat\nunsat\n",
	"solve_eqs_boolean_scopes.smt2":                "sat\n((a true) (b true))\nunsat\nsat\n((a true) (b true))\n",
	"solve_eqs_real_definitions.smt2":              "sat\n((x 2.0) (y 1.0))\n",
	"solve_eqs_ite_definitions.smt2":               "sat\n(((ite c (= x 2) (= x 3)) true))\n",
	"solve_eqs_core.smt2":                          "unsat\n(positive equality negative)\n",
	"solve_eqs_cycle.smt2":                         "sat\n(((= a (f b)) true) ((= b (f a)) true))\n",
	"solve_eqs_as_array_scopes.smt2":               "sat\nunsat\nsat\n",
	"solve_eqs_proof.smt2":                         "unsat\n",
	"propagate_values_model_scopes.smt2":           "sat\n((a true) (b false) (c true))\nunsat\nsat\n((a true) (b false) (c true))\n",
	"propagate_values_core.smt2":                   "unsat\n(negative implication positive)\n",
	"propagate_values_proof.smt2":                  "unsat\n",
	"tactic_model_scopes.smt2":                     "sat\n((a true))\nunsat\nsat\n((a true))\nsat\n((a true))\n",
	"tactic_unsat_core.smt2":                       "unsat\n(positive negative)\nsat\n",
	"tactic_false_proof.smt2":                      "unsat\n((set-logic QF_UF)\n(proof\n(asserted false)))\n",
	"tactic_pretty_proof.smt2":                     "unsat\n(asserted false)\n",
	"tactic_unknown_recovery.smt2":                 "unknown\nunsat\nunsat\n",
	"tactic_failure_recovery.smt2":                 "unknown\nsat\nsat\n((a true))\n",
	"tactic_resource_limit_recovery.smt2":          "unknown\nsat\nsat\n((a true))\n",
	"array_as_array_direct_core_scopes.smt2":       "unknown\nunknown\nunknown\n",
	"model_macro_cleanup_scopes.smt2":              "sat\n((c false) ((f false) false) ((f true) false))\nsat\n((c true) ((f false) false) ((f true) true))\nsat\n((c false) ((f false) false) ((f true) false))\n",
	"model_compact_live_updates.smt2":              "sat\n(model (define-fun f ((x!0 Bool)) Bool (ite (= x!0 false) false (ite (= x!0 true) false false)))\n)\nsat\n(model (define-fun f ((x!0 Bool)) Bool false)\n)\n(((f false) false) ((f true) false))\nsat\n(model (define-fun f ((x!0 Bool)) Bool (ite (= x!0 false) false (ite (= x!0 true) false false)))\n)\n",
	"array_explicit_extensionality_scopes.smt2":    "sat\nunsat\nsat\nunsat\nsat\n",
	"array_default_store_visitation_scopes.smt2":   "sat\nsat\nunsat\nsat\n",
	"array_function_reference_signatures.smt2":     "unsat\nunsat\nunsat\nsat\n",
	"array_canonical_constructor_reads.smt2":       "sat\n(((select a i) true) ((select b j) false))\nunsat\nsat\nunsat\nunsat\nsat\n(((select a i) true) ((select b j) false))\n",
	"array_constructor_default_models.smt2":        "sat\n(((= a ((as const (Array Bool Bool)) true)) true) ((= b (store a false false)) true) ((select a true) true) ((select b false) false) ((select b true) true))\nunsat\nsat\n(((= a ((as const (Array Bool Bool)) true)) true) ((= b (store a false false)) true) ((select a true) true) ((select b false) false) ((select b true) true))\n",
	"array_store_chain_model_propagation.smt2":     "sat\n(((= b (store a true false)) true) ((= c (store b false true)) true) ((select b false) true) ((select c false) true) ((select c true) false))\nunsat\nsat\n(((= b (store a true false)) true) ((= c (store b false true)) true) ((select b false) true) ((select c false) true) ((select c true) false))\n",
	"array_cyclic_store_model_scopes.smt2":         "sat\n(((select a true) true) ((= a (store a true true)) true))\nunsat\nsat\n(((select a true) true) ((= a (store a true true)) true))\n",
	"array_weak_live_parent_scopes.smt2":           "sat\n(((= a b) true))\nunsat\nunsat\nsat\n(((= a b) true))\n",
	"array_late_default_alias_scopes.smt2":         "sat\nunsat\nsat\n",
	"array_multidimensional_read_queue.smt2":       "sat\nunsat\nsat\nunsat\nsat\n",
	"array_multidimensional_read_queue_proof.smt2": "sat\nunsat\nsat\nunsat\nsat\n",
	"array_nested_interface_scopes.smt2":           "sat\nunsat\nsat\n",
	"array_extensional_disequality_timing.smt2":    "sat\nunsat\nsat\nsat\nsat\n",
	"array_simple_boolean_witness_scopes.smt2":     "sat\n(((distinct a b) true))\nunsat\nsat\n(((distinct a b) true))\n",
	"array_full_boolean_witness_scopes.smt2":       "sat\n(((distinct a b) true))\nunsat\nsat\n(((distinct a b) true))\n",
	"array_extensional_simple_live_updates.smt2":   "sat\nunsat\nsat\nsat\n",
	"array_extensional_live_updates.smt2":          "sat\nunsat\nsat\nsat\n",
	"array_extensional_proof_scopes.smt2":          "unsat\nsat\n",
	"array_extensional_disabled_scopes.smt2":       "sat\nsat\n",
	"array_extensional_enabled_scopes.smt2":        "unsat\nsat\n",
	"bit_vector_symbolic_carry_scopes.smt2":        "sat\n(((= (bvadd x y) #xa) true) ((= (bvsub x y) #x4) true))\nunsat\nsat\n(((= (bvadd x y) #xa) true) ((= (bvsub x y) #x4) true))\n",
	"utvpi_mixed_model_refusal.smt2":               "unknown\n",
	"utvpi_mixed_sum_model_refusal.smt2":           "unknown\n(:reason-unknown \"invalid rational value passed as an integer\")\n",
	"utvpi_integer_parity_scopes.smt2":             "unsat\nsat\n((x 1) (y 1))\nunsat\nsat\n((x 1) (y 1))\n",
	"utvpi_mixed_models.smt2":                      "sat\n(((>= i 0) true) ((<= i 1) true) ((> r 0.0) true) ((< r (/ 1.0 3.0)) true))\nunsat\nsat\n(((>= i 0) true) ((<= i 1) true) ((> r 0.0) true) ((< r (/ 1.0 3.0)) true))\n",
	"utvpi_real_strict_scopes.smt2":                "sat\n(((> (+ x y) 0.0) true) ((< (+ x y) (/ 1.0 3.0)) true) ((= x y) true))\nunsat\nsat\n(((> (+ x y) 0.0) true) ((< (+ x y) (/ 1.0 3.0)) true) ((= x y) true))\n",
	"utvpi_unknown_first_push.smt2":                "sat\n((x (/ 1.0 2.0)) (y (/ 1.0 2.0)))\nunsat\nsat\n((x (/ 1.0 2.0)) (y (/ 1.0 2.0)))\n",
	"basic_graph_integer.smt2":                     "sat\n((i 1))\nunsat\nsat\n((i 1))\n",
	"basic_graph_real.smt2":                        "sat\n(((> r 0.0) true) ((< r (/ 1.0 3.0)) true))\nunsat\nsat\n(((> r 0.0) true) ((< r (/ 1.0 3.0)) true))\n",
	"basic_dense_mixed.smt2":                       "sat\n(((>= i 0) true) ((<= i 1) true) ((> r 0.0) true) ((< r (/ 1.0 3.0)) true))\nunsat\nsat\n(((>= i 0) true) ((<= i 1) true) ((> r 0.0) true) ((< r (/ 1.0 3.0)) true))\n",
	"basic_dense_forced_equality.smt2":             "sat\nunsat\nsat\n",
	"basic_disabled_arithmetic_scopes.smt2":        "sat\n((p true))\nunknown\nsat\n((p true))\n",
	"basic_graph_mixed_model_refusal.smt2":         "unknown\n",
	"basic_unknown_first_push_dense.smt2":          "sat\n((r (/ 1.0 3.0)))\nunsat\nsat\n((r (/ 1.0 3.0)))\n",
	"ufidl_congruence_equality_bounds.smt2":        "sat\n(((>= (f u) 3) true) ((<= (f v) 2) true) ((distinct u v) true))\nunsat\nsat\n(((>= (f u) 3) true) ((<= (f v) 2) true) ((distinct u v) true))\n",
	"ufidl_dense_model_scopes.smt2":                "sat\n((x 6) (y 2))\nunsat\nsat\n((x 6) (y 2))\n",
	"ufidl_congruence_models.smt2":                 "sat\n(((f x) 5) ((f y) 7) ((distinct x y) true))\nunsat\nsat\n(((f x) 5) ((f y) 7) ((distinct x y) true))\n",
	"rdl_dense_model_scopes.smt2":                  "sat\n((x (/ 3.0 2.0)) (y (/ 1.0 2.0)))\nunsat\nsat\n((x (/ 3.0 2.0)) (y (/ 1.0 2.0)))\n",
	"rdl_graph_strict_scopes.smt2":                 "sat\n(((> x 0.0) true) ((< x (/ 1.0 3.0)) true))\nunsat\nsat\n(((> x 0.0) true) ((< x (/ 1.0 3.0)) true))\n",
	"idl_dense_model_scopes.smt2":                  "sat\n((x 6) (y 2))\nunsat\nsat\n((x 6) (y 2))\n",
	"idl_first_push_simplex.smt2":                  "sat\n((x 3) (y 1))\nunsat\nsat\n((x 3) (y 1))\n",
	"bit_vector_auto_carry.smt2":                   "sat\n(((bvadd x y) #xa) ((bvsub x y) #x4))\nunsat\nsat\n(((bvadd x y) #xa) ((bvsub x y) #x4))\n",
	"lra_auto_ite_profile.smt2":                    "sat\n((x (/ 3.0 2.0)))\nunsat\nsat\n((x (/ 3.0 2.0)))\n",
	"quantified_mixed_auto_profile.smt2":           "sat\n(((= (f 3.0) 4.0) true))\nunsat\nsat\n(((= (f 7.0) 8.0) true))\n",
	"array_auto_unit_profile.smt2":                 "sat\n(((select a true) true))\nsat\n(((select a true) true) ((select a false) false))\nsat\n(((select a true) true))\nunsat\nsat\n(((select a true) true))\n",
	"array_model_relevancy.smt2":                   "sat\n(((select a 3) 7) ((select a 99) 11))\nsat\n(((select a 3) 7) ((select a 4) 19) ((select a 99) 11))\nsat\n(((select a 3) 7) ((select a 99) 11))\nunsat\nsat\n(((select a 3) 7) ((select a 99) 11))\n",
	"real_auto_rational_profile.smt2":              "sat\n(((= x threshold) true))\nunsat\nsat\n(((= x threshold) true))\n",
	"integer_auto_deep_ite.smt2":                   "sat\n((holds true))\nunsat\nsat\n((holds true))\n",
	"quantified_ematching_saturation.smt2":         "unknown\nunsat\nunknown\n",
	"quantified_ematching_relevancy.smt2":          "unsat\nunsat\n",
	"quantified_model_relevancy.smt2":              "sat\n(((f 7) 7) ((f 11) 11))\nunsat\nsat\n(((f 11) 11))\n",
	"integer_uf_model_relevancy.smt2":              "sat\n(((distinct x y) true) ((distinct (f x) (f y)) true))\nunsat\nsat\n(((distinct x y) true) ((distinct (f x) (f y)) true))\n",
	"arithmetic_eq_options.smt2":                   "unsat\nsat\nunsat\nsat\nunsat\nsat\n",
	"rational_square_interval_scopes.smt2":         "unsat\nsat\nsat\n((d (/ 121.0 225.0)) (|squared difference| (/ 121.0 225.0)) ((let ((q d)) (+ q 0)) (/ 121.0 225.0)))\nunsat\nsat\nsat\n",
	"shared_bound_queue.smt2":                      "unknown\nsat\n((x 2.0) ((f x) 0.0) ((f z) 0.0))\nsat\n",
	"shared_nonlinear_model.smt2":                  "sat\nunknown\nsat\nsat\n((x 2.0) (y 1.0) (z (- 1.0)) ((f z) 0.0))\n",
	"row_conflict_relaxation.smt2":                 "unsat\nsat\nsat\n",
	"grobner_factor_dependencies.smt2":             "unsat\nsat\nsat\n",
	"nested_bound_dependency_scopes.smt2":          "unsat\nsat\nunsat\nsat\n",
	"nonlinear_bound_provenance_scopes.smt2":       "sat\nunsat\nsat\nunsat\nsat\n",
	"integer_bound_provenance_scopes.smt2":         "unsat\nunsat\nsat\n",
	"mixed_cut_bound_provenance_scopes.smt2":       "unsat\nsat\n",
	"integer_euclidean_scopes.smt2":                "unsat\nsat\n((x 1) (y 1))\n",
	"mixed_euclidean_large_integer.smt2":           "sat\n((x 1) (y 1))\n",
	"integer_equality_branch_scopes.smt2":          "unsat\nsat\n((x 1) (y 1))\n",
	"integer_branch_cut_scopes.smt2":               "sat\n((x 0) (y 1))\nunsat\nsat\n",
	"mixed_gomory_scopes.smt2":                     "unsat\nsat\n",
	"integer_freedom_patch_scopes.smt2":            "sat\n((x 1) (y 2))\nunsat\nsat\n",
	"mixed_integer_freedom_patch.smt2":             "sat\n((x 1) (y 2))\nsat\nunsat\nsat\n",
	"shared_linear_expression_bounds.smt2":         "unsat\nunsat\nsat\n",
	"row_bound_propagation_scopes.smt2":            "unsat\n(x_strict y_lower blocked_sum)\nsat\n((x (/ 3.0 2.0))\n (y 2.0)\n ((+ x y) (/ 7.0 2.0)))\n",
	"sequence_word_equations_relations.smt2":       "unsat\nsat\n((x \"aa\"))\nsat\n((x \"ab\"))\nsat\n((x \"bc\"))\nsat\n((x \"ab\"))\nsat\nunsat\nsat\nsat\nsat\nsat\nsat\nsat\n",
	// sequence_helper_inventory: excluded, g3 fixes a z3 4.7.1 automaton product bug (test_vectors/z3_bug_exclusions.txt).
	// "sequence_helper_inventory.smt2":               "unsat\nsat\nunsat\nsat\nunsat\nunsat\nunsat\nunsat\nunsat\nunsat\nunsat\nunsat\nsat\n",
	"mixed_equality_and_ite_join.smt2":                "sat\nunsat\nunsat\nunsat\nsat\n",
	"integer_gcd_scopes.smt2":                         "unsat\nsat\n",
	"integer_extended_gcd.smt2":                       "unsat\nsat\n",
	"mixed_integer_gcd_scopes.smt2":                   "sat\nunsat\nunsat\nsat\n",
	"nonlinear_nested_row_aliases.smt2":               "unsat\nsat\n",
	"nonlinear_nested_integer_row_aliases.smt2":       "unsat\nsat\n",
	"shared_arithmetic_equality_cycle.smt2":           "sat\nunsat\nsat\n",
	"shared_arithmetic_integer_cycle.smt2":            "sat\nunsat\nsat\n",
	"nonlinear_max_min_bounds.smt2":                   "unsat\n",
	"nonlinear_max_min_model.smt2":                    "sat\n",
	"shared_arithmetic_fixed_rows.smt2":               "unsat\nsat\n",
	"shared_arithmetic_fixed_integer.smt2":            "unsat\n",
	"nonlinear_numeric_progress.smt2":                 "unsat\nsat\n((x 3) (y 4))\n",
	"nonlinear_cluster_inequalities.smt2":             "unsat\nsat\n((x 1.0) (y 1.0) (z (- 1.0)))\n",
	"nonlinear_cluster_strict.smt2":                   "unsat\n",
	"nonlinear_horner_bound.smt2":                     "unsat\n",
	"nonlinear_completed_square.smt2":                 "unsat\n",
	"nonlinear_nested_multivariate.smt2":              "unsat\nsat\n",
	"nonlinear_nested_integer.smt2":                   "unsat\n",
	"nested_rational_high_degree.smt2":                "unsat\nunsat\nsat\n((x 1.0)\n (y 2.0)\n ((+ (* (/ 1 2) x x y) (* (- 1) x y) (* (/ 3 2) y)) 2.0))\n",
	"linearized_product_real_scopes.smt2":             "sat\n((y 1.0))\nsat\n((y (/ 1.0 2.0)))\nsat\n((y (- 1.0)))\n",
	"linearized_product_integer_scopes.smt2":          "unsat\nsat\n((y 1))\n",
	"nonlinear_integer_factor_scopes.smt2":            "unsat\nsat\n",
	"nonlinear_integer_zero_split.smt2":               "sat\n",
	"nonlinear_integer_upper_split.smt2":              "sat\n",
	"nonlinear_grobner_expanded_square.smt2":          "unsat\nsat\n",
	"nonlinear_grobner_rational_square.smt2":          "unsat\n",
	"nonlinear_grobner_coupled.smt2":                  "unsat\nsat\n",
	"nonlinear_zero_candidate.smt2":                   "sat\n((x 0.0) ((* x x) 0.0))\n",
	"nonlinear_interval_candidate.smt2":               "sat\n",
	"nonlinear_product_scopes.smt2":                   "unsat\nsat\n((x 2.0) (y 3.0) ((* x y) 6.0))\n",
	"nonlinear_positive_product.smt2":                 "unsat\n",
	"integer_zero_divisor.smt2":                       "sat\n(((div x 0) (div 1 0)) ((mod x 0) (mod 1 0)) ((rem x 0) (rem 1 0)))\n",
	"integer_division_model.smt2":                     "sat\n((x (- 7)) ((div x 3) (- 3)) ((mod x 3) 2) ((rem x 3) 2))\n",
	"integer_negative_divisor.smt2":                   "sat\n((x (- 7)) ((div x (- 3)) 3) ((mod x (- 3)) 2) ((rem x (- 3)) (- 2)))\n",
	"integer_division_scopes.smt2":                    "unsat\nsat\n((x 5))\n",
	"mixed_floor_scopes.smt2":                         "sat\n((i (- 2)))\nunsat\nsat\n",
	"mixed_is_int.smt2":                               "unsat\n",
	"mixed_not_is_int.smt2":                           "unsat\n",
	"mixed_arithmetic_model.smt2":                     "sat\n((i 1) (r (/ 3.0 2.0)))\n",
	"mixed_integer_fraction.smt2":                     "unsat\n",
	"mixed_negative_epsilon.smt2":                     "unsat\n",
	"mixed_arithmetic_scopes.smt2":                    "unsat\nsat\n((i 1) (r (/ 1.0 2.0)))\n",
	"all_quantified_arithmetic_after_push.smt2":       "unsat\nsat\n",
	"quantified_bounds.smt2":                          "unsat\nsat\n",
	"bit_vector_sharing_scopes.smt2":                  "unsat\nsat\nunsat\nsat\n",
	"inferred_trigger.smt2":                           "unsat\n",
	"function_overloads.smt2":                         "sat\n",
	"function_definitions.smt2":                       "unsat\n",
	"definition_scopes.smt2":                          "sat\nsat\n",
	"definition_capture.smt2":                         "unsat\n",
	"definition_coercions.smt2":                       "unsat\n",
	"function_coercions.smt2":                         "unsat\n",
	"global_definitions.smt2":                         "unsat\n",
	"boolean_model.smt2":                              "sat\n(model\n  (define-fun p () Bool true)\n)\n",
	"constant_array.smt2":                             "unsat\n",
	"string_value.smt2":                               "sat\n((x \"a\"\"b\"))\n",
	"global_declarations.smt2":                        "sat\nsat\n",
	"boolean_scopes.smt2":                             "sat\n((|p spaced| true))\nunsat\nsat\na\"b\n",
	"uf_congruence.smt2":                              "unsat\n",
	"linear_integer.smt2":                             "sat\n((x 4))\nunsat\nsat\n",
	"linear_real.smt2":                                "sat\n((x (/ 3.0 2.0)))\n",
	"pareto_optimization.smt2":                        "sat\n(objectives\n (x 3)\n (y 3)\n)\n((x 3)\n (y 3))\nunsat\nsat\n(objectives\n (x 3)\n (y 3)\n)\n((x 3)\n (y 3))\n",
	"pareto_bit_vector_optimization.smt2":             "sat\n(objectives\n (x 3)\n (y 3)\n)\n((x #b11)\n (y #b11))\nunsat\nsat\n(objectives\n (x 3)\n (y 3)\n)\n((x #b11)\n (y #b11))\n",
	"pareto_soft_optimization.smt2":                   "sat\n(objectives\n (x 0)\n (y 0)\n)\n((a true)\n (b true))\nunsat\nsat\n(objectives\n (x 0)\n (y 0)\n)\n((a true)\n (b true))\n",
	"pareto_single_negative_soft.smt2":                "sat\n(objectives\n ( (- 1))\n)\nsat\n(objectives\n ( (- 1))\n)\nsat\n",
	"pareto_single_objective_negative_softs.smt2":     "sat\n(objectives\n ( (- 3))\n)\n((x 0)\n (a false))\nsat\n(objectives\n ( (- 3))\n)\nsat\n",
	"box_single_objective_rechecks.smt2":              "sat\n(objectives\n (x 4)\n)\nsat\n(objectives\n (x 4)\n)\nsat\n",
	"pareto_soft_minimize_push_pop.smt2":              "sat\n(objectives\n (p (- 3))\n (q 0)\n)\nsat\n(objectives\n (p (- 3))\n (q 0)\n)\nsat\n(objectives\n (p (- 3))\n (q 0)\n (x 3)\n)\n",
	"box_soft_minimize_push_pop.smt2":                 "sat\n(objectives\n (p (- 3))\n (q (- 3))\n)\nsat\n(objectives\n (p (- 3))\n (q (- 3))\n)\nsat\n(objectives\n (p (- 3))\n (q (- 3))\n (x (* (- 1) oo))\n)\n",
	"pareto_soft_two_maximize.smt2":                   "sat\n(objectives\n (p 0)\n (x 4)\n (y 1)\n)\n(model \n  (define-fun x () Int\n    4)\n  (define-fun y () Int\n    1)\n  (define-fun a () Bool\n    true)\n)\nsat\n(objectives\n (p 0)\n (x 1)\n (y 4)\n)\nsat\n(objectives\n (p 0)\n (x 2)\n (y 3)\n)\nsat\n",
	"box_int_real_four_objectives.smt2":               "sat\n(objectives\n (x 0)\n (y 50)\n (z (+ (/ 15.0 2.0) (* (- 1.0) epsilon)))\n ((+ x y) (- 3))\n)\n(model \n  (define-fun z () Real\n    0.0)\n  (define-fun y () Int\n    (- 3))\n  (define-fun x () Int\n    0)\n)\nsat\n(objectives\n (x 0)\n (y 50)\n (z (+ (/ 15.0 2.0) (* (- 1.0) epsilon)))\n ((+ x y) (- 3))\n)\nsat\n(objectives\n (x 0)\n (y 50)\n (z (+ (/ 15.0 2.0) (* (- 1.0) epsilon)))\n ((+ x y) (- 3))\n)\n",
	"pareto_weighted_soft_minimize_rechecks.smt2":     "sat\n(objectives\n (p 2)\n (q 0)\n (x (- 2))\n)\nsat\n(objectives\n (p 0)\n (q 3)\n (x 3)\n)\nunsat\n(objectives\n (p 0)\n (q 3)\n (x 3)\n)\nsat\n(objectives\n (p 2)\n (q 0)\n (x (- 2))\n)\nsat\n(objectives\n (p 2)\n (q 0)\n (x (- 2))\n)\nsat\n(objectives\n (p 2)\n (q 0)\n (x (- 2))\n)\n",
	"lex_then_box_soft_arith.smt2":                    "sat\n(objectives\n (p 0)\n ((- x y) 5)\n (y 4)\n)\nsat\n(objectives\n (p 0)\n ((- x y) 9)\n (y 0)\n)\n",
	"pareto_real_two_maximize.smt2":                   "sat\n(objectives\n (x (/ 7.0 4.0))\n ((* 2.0 y) (/ 7.0 2.0))\n)\nsat\n(objectives\n (x (/ 5.0 4.0))\n ((* 2.0 y) (/ 9.0 2.0))\n)\nsat\n",
	"box_same_term_three_objectives.smt2":             "sat\n(objectives\n (y 99)\n (y (* (- 1) oo))\n ((- x y) oo)\n)\nsat\n",
	"box_soft_minmax_push_pop.smt2":                   "sat\n(objectives\n (p 0)\n (q 0)\n (x (* (- 1) oo))\n (y 19)\n)\nsat\n(objectives\n (p 0)\n (q 0)\n (x (* (- 1) oo))\n (y 19)\n)\nsat\n(objectives\n (p 0)\n (q 0)\n (x 1)\n (y 19)\n)\nsat\n",
	"lex_unexecuted_soft_offset_interval.smt2":        "sat\n(objectives\n (x 5)\n (y oo)\n (q  (interval (- 1) 1))\n)\n",
	"lex_unexecuted_softs_interval_continuation.smt2": "sat\n(objectives\n (p 0)\n (x 7)\n (y oo)\n (q  (interval (- 3) (- 1)))\n (s  (interval 0 (/ 3.0 2.0)))\n)\nsat\n(objectives\n (p 0)\n (x 7)\n (y oo)\n (q  (interval (- 3) (- 1)))\n (s  (interval 0 (/ 3.0 2.0)))\n)\n",
	"box_farkas_engine.smt2":                          "sat\n(objectives\n (x 0)\n (y 0)\n (z 0)\n ((+ x y) 0)\n)\nsat\n(objectives\n (x 0)\n (y 0)\n (z 0)\n ((+ x y) 0)\n)\n",
	"box_symba_engine.smt2":                           "sat\n(objectives\n (x 0)\n (y 50)\n (z (+ (/ 15.0 2.0) (* (- 1.0) epsilon)))\n ((+ x y) (- 3))\n)\nsat\n(objectives\n (x 0)\n (y 50)\n (z (+ (/ 15.0 2.0) (* (- 1.0) epsilon)))\n ((+ x y) (- 3))\n)\n",
	"box_farkas_real_unknown_bounds.smt2":             "unknown\n(objectives\n (x  (interval 0 oo))\n (y  (interval 1 oo))\n)\n(model \n  (define-fun y () Real\n    1.0)\n  (define-fun x () Real\n    0.0)\n)\nunknown\nunsat\n",
	"lex_symba_unknown_bounds.smt2":                   "unknown\n(objectives\n (x  (interval 0 oo))\n (y  (interval 0 oo))\n)\n",
	"lex_farkas_unknown_bounds.smt2":                  "unknown\n(objectives\n (x  (interval 0 oo))\n (y  (interval 0 oo))\n)\n",
	"simplify_command.smt2":                           "(+ 5 x)\n(+ (* x x) (* 2 x y) (* y y))\ntrue\n3\n(rewrite (= (+ 1 2) 3))\n",
	"simplify_sequence_command.smt2":                  "(= c #x61)\n",
	"pending_default_portfolio_option_error.smt2":     "unknown\n",
	"get_assertions.smt2":                             "((and a true))\n((and a true)\n (! b :named n))\n((and a true))\n",
	"get_assignment.smt2":                             "sat\n((|p q| true) (q false))\nsat\n((|p q| true) (q false) (r false))\nsat\n((|p q| true) (q false))\n",
	"get_unsat_assumptions.smt2":                      "unsat\n(a)\nunsat\n(b)\nunsat\n((not b))\nunsat\n(a)\n",
	"get_consequences.smt2":                           "sat\n\n(=> a a)\nsat\n\n(=> a (= x 3))\nsat\n\n(=> true (= y 4))\nsat\n\n(=> true (= y 4))\nunsat\n\n",
	"get_option.smt2":                                 "17\ntrue\nfalse\ntrue\ntrue\ntrue\n123\n0\n2\ncontinued-execution\ntrue\n",
	"get_info.smt2":                                   "(:name \"Z3\")\n(:authors \"Leonardo de Moura, Nikolaj Bjorner and Christoph Wintersteiger\")\n(:version \"4.7.1\")\n(:error-behavior continued-execution)\n(:status unknown)\n(:assertion-stack-levels 0)\n(:status sat)\n(:assertion-stack-levels 2)\n(:status sat)\n(:assertion-stack-levels 0)\n",
	"labels.smt2":                                     "sat\n(labels)\nunknown\n(labels)\n",
	"expression_labels.smt2":                          "sat\n(labels)\nsat\nsat\n(labels)\n",
	"display.smt2":                                    "(+ x 1)\n(and true false)\n",
	"declare_map.smt2":                                "unsat\n",
	"interpolation_commands.smt2":                     "unsat\n(interpolants\n q)\nsat\nunsat\n(interpolants\n p)\nunsat\n(interpolants\n p)\nunsat\n(interpolants\n (not (>= x 0)))\n",
	"lexicographic_integer_optimization.smt2":         "sat\n(objectives\n (x 3)\n (y 2)\n)\n((x 3)\n (y 2))\nsat\n(objectives\n (x 0)\n (y 5)\n)\n((x 0)\n (y 5))\n",
	"bit_vector.smt2":                                 "sat\n((x #xf))\n",
	"bit_vector_optimization.smt2":                    "sat\n(objectives\n (x 9)\n)\n((x #x9))\nsat\n(objectives\n (x 3)\n)\n((x #x3))\nsat\n(objectives\n (x 15)\n (y 8)\n)\n((x #xf)\n (y #x8))\n",
	"box_optimization.smt2":                           "sat\n(objectives\n (x 5)\n (y 5)\n)\n((x 5)\n (y 0))\nsat\n(objectives\n (x 5)\n (y 5)\n)\n((x 0)\n (y 5))\nunsat\n",
	"bounded_integer_optimization.smt2":               "sat\n(objectives\n (x 3)\n)\n((x 3))\nsat\n(objectives\n (x 0)\n)\n((x 0))\n",
	"bounded_real_strict_optimization.smt2":           "sat\n(objectives\n (x (+ 5.0 (* (- 1.0) epsilon)))\n)\n((x (/ 9.0 2.0)))\nsat\n(objectives\n (x epsilon)\n)\n((x (/ 1.0 2.0)))\n",
	"strict_lexicographic_interval.smt2":              "sat\n(objectives\n (x  (interval (/ 9.0 2.0) (+ 5.0 (* (- 1.0) epsilon))))\n (y (/ 9.0 2.0))\n)\n((x (/ 9.0 2.0))\n (y (/ 9.0 2.0)))\n",
	"array_read_store.smt2":                           "unsat\n",
	"quantified_boolean.smt2":                         "unsat\n",
	"parallel_let.smt2":                               "sat\n((p true))\n",
	"declaration_scopes.smt2":                         "sat\nsat\n",
	"named_core.smt2":                                 "unsat\n(positive negative)\n",
	"quantified_let_capture.smt2":                     "unsat\n",
	"indexed_bit_vector.smt2":                         "unsat\n",
	"nary_equality.smt2":                              "unsat\n",
	"unknown_static_classifier.smt2":                  "sat\n((xr 1.0)\n (yr 2.0))\nsat\n((xi 2)\n (yi 3))\nsat\n((ur 4.0)\n ((fr ur) 7.0))\nsat\n((ui 6)\n ((fi ui) 9))\n",
	"unbounded_real_optimization.smt2":                "sat\n(objectives\n (x oo)\n)\n((x 0.0))\nsat\n(objectives\n (x (* (- 1) oo))\n)\n((x 0.0))\n",
	"unbounded_lexicographic_interval.smt2":           "sat\n(objectives\n (x oo)\n (y  (interval (* (- 1) oo) oo))\n)\n((x 0.0)\n (y 0.0))\n",
	"weighted_soft_optimization.smt2":                 "sat\n(objectives\n ( 1)\n)\n((a false)\n (b true))\nsat\n(objectives\n (first 0)\n (second (/ 9.0 4.0))\n)\n((a true)\n (b false))\nsat\n(objectives\n (second (- 2))\n)\n((a false))\nsat\n(objectives\n (soft-first (- 2))\n (x 1)\n (soft-last 0)\n)\n((a false)\n (x 1))\nsat\n(objectives\n (duplicate 2)\n)\n((a true))\nsat\n(objectives\n (zero 0)\n)\n",

	"fixedpoint_pdr_untransformed_linear_trace.smt2":    "sat\n(and Goal (P 0 7))\n",
	"fixedpoint_pdr_untransformed_nonlinear_trace.smt2": "sat\n(and Goal (P 0) (Q 0))\n",
	"fixedpoint_pdr_utvpi_fragment.smt2":                "unsat\n",
	"fixedpoint_pdr_general_arithmetic_fragment.smt2":   "unsat\n",
	"fixedpoint_pdr_arithmetic_inductive.smt2":          "unsat\n",
	"fixedpoint_pdr_multicore.smt2":                     "unsat\n",
	"fixedpoint_pdr_sliced_trace.smt2":                  "sat\n(and Goal (P!slice!0 0))\n",
	"fixedpoint_pdr_sliced_compound_tail.smt2":          "sat\n(and Goal (P!slice!0 0))\n",
	"fixedpoint_pdr_sliced_compound_head.smt2":          "unsat\n",
	"fixedpoint_pdr_inductive_reachability.smt2":        "sat\ntrue\n",
	"fixedpoint_pdr_bit_blast.smt2":                     "sat\n",
	"fixedpoint_pdr_scale.smt2":                         "sat\n",
	"sortmax_soft_hash_order.smt2":                      "sat\n(objectives\n ( 0)\n)\n",
	"setup_unknown_qf_fp_with_int.smt2":                 "sat\n(model \n  (define-fun f () (_ FloatingPoint 3 5)\n    (fp #b0 #b010 #x0))\n  (define-fun y () Int\n    1)\n  (define-fun x () Int\n    1)\n)\n",
	"setup_basic_qf_fp_with_int.smt2":                   "sat\n(model \n  (define-fun f () (_ FloatingPoint 3 5)\n    (fp #b0 #b010 #x0))\n  (define-fun y () Int\n    1)\n  (define-fun x () Int\n    1)\n)\n",
	"maxres_single_correction_set.smt2":                 "sat\n(objectives\n ( 3)\n)\n(model \n  (define-fun y () Int\n    0)\n  (define-fun x () Int\n    8)\n)\n",
	"setup_unknown_uflra_ground.smt2":                   "sat\n(model \n  (define-fun y () Real\n    (- (/ 5.0 2.0)))\n  (define-fun x () Real\n    (- 5.0))\n  (define-fun f ((x!0 Real)) Real\n    (ite (= x!0 (- 5.0)) (- (/ 1.0 2.0))\n    (ite (= x!0 (- (/ 5.0 2.0))) (- (/ 15.0 2.0))\n      (- (/ 1.0 2.0)))))\n)\n",
}

func TestSMTLibRunnerScripts(t *testing.T) {
	for name, expected := range scriptResults {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err = run(bytes.NewReader(script), &out)
			if sourceDebugMacroCoercionRefusal(t, name, err, &out) {
				return
			}
			if sourceDebugModusPonensRefusal(t, name, err, &out) {
				return
			}
			if err != nil {
				t.Fatal(err, out.String())
			}
			if canonicalReferenceOutput(name, out.String()) != canonicalReferenceOutput(name, expected) {
				t.Fatal("Go SMT-LIB result", out.String(), "want", expected)
			}
		})
	}
}

// The native executable is a release build. With proofs, solve-eqs builds
// mk_modus_ponens(asserted a, trans(a = b, b iff false)) whose second fact is
// (= a false), an OP_EQ; ast.cpp:2644 asserts it is implies, iff or oeq in
// source debug builds.
func sourceDebugModusPonensRefusal(t *testing.T, name string, err error, out *bytes.Buffer) bool {
	t.Helper()
	if !ast.ASTSourceDebug || name != "solve_eqs_proof.smt2" {
		return false
	}
	want := "line 8 column 26: " + ast.ErrASTDomain.Error()
	if err == nil || err.Error() != want || out.String() != "(error \""+want+"\")\n" {
		t.Fatalf("source debug modus ponens assertion err=%q output=%q", err, out.String())
	}
	return true
}

// The native executable is a release build. Its macro substitution accepts
// Int arguments in Real binders, while rewriter_def.h:49 asserts exact binder
// and replacement sorts in source debug builds, before application coercion.
func sourceDebugMacroCoercionRefusal(t *testing.T, name string, err error, out *bytes.Buffer) bool {
	t.Helper()
	if !ast.ASTSourceDebug || name != "definition_coercions.smt2" {
		return false
	}
	wantOutput := "(error \"line 3 column 26: " + ast.ErrASTDomain.Error() + "\")\nsat\n"
	if err == nil || err.Error() != "line 3 column 26: "+ast.ErrASTDomain.Error() || out.String() != wantOutput {
		t.Fatalf("source debug macro-binding sort assertion err=%q output=%q", err, out.String())
	}
	return true
}

// Explicitly opt in to the repository's C++ executable. Ordinary Go semantic
// tests use checked-in vectors and do not require a native reference build.
func TestSMTLibRunnerCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the C++ z3 executable")
	}
	for name := range scriptResults {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			var goOutput bytes.Buffer
			err = run(bytes.NewReader(script), &goOutput)
			debugRefusal := sourceDebugMacroCoercionRefusal(t, name, err, &goOutput)
			if err != nil && !debugRefusal {
				t.Fatal(err, goOutput.String())
			}
			cmd := exec.Command(reference, "-in")
			var referenceContext context.Context
			if name == "recursive_mutual_boolean_model.smt2" {
				var cancel context.CancelFunc
				referenceContext, cancel = context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				cmd = exec.CommandContext(referenceContext, reference, "-in")
			}
			cmd.Stdin = bytes.NewReader(script)
			var cppDiagnostics bytes.Buffer
			cmd.Stderr = &cppDiagnostics
			cppOutput, err := cmd.Output()
			if referenceContext != nil && referenceContext.Err() == context.DeadlineExceeded {
				// The old reference eagerly evaluates a dead recursive OR branch
				// in Boolean model interpretations. Keep the full native oracle,
				// and require the precise observed reference prefix on timeout.
				if canonicalOutput(string(cppOutput)) != "sat" || canonicalOutput(goOutput.String()) != canonicalOutput(scriptResults[name]) {
					t.Fatal("unexpected Boolean recursive reference failure", err, string(cppOutput), goOutput.String())
				}
				t.Log("known reference Boolean-recursive model-evaluation loop; integer mutual recursion remains an exact C++ oracle")
				return
			}
			if err != nil {
				t.Fatal("C++ reference", err, string(cppOutput), cppDiagnostics.String())
			}
			if debugRefusal {
				if canonicalOutput(string(cppOutput)) != canonicalOutput(scriptResults[name]) {
					t.Fatal("native macro coercion result", string(cppOutput))
				}
				return
			}
			if name == "bv_native_operator_models.smt2" {
				// The source constant folder treats signed #xa as +10. Go
				// corrects the overflow predicate for -6 * 3, which has no
				// positive overflow (but does have signed underflow).
				cppOutput = bytes.Replace(cppOutput, []byte("((bvsmul_noovfl a b) false)"), []byte("((bvsmul_noovfl a b) true)"), 1)
			}
			if canonicalReferenceOutput(name, goOutput.String()) != canonicalReferenceOutput(name, string(cppOutput)) {
				t.Fatalf("Go/C++ mismatch\nGo:\n%s\nC++:\n%s\nC++ diagnostics:\n%s", goOutput.String(), cppOutput, cppDiagnostics.String())
			}
		})
	}
}

func canonicalReferenceOutput(name, output string) string {
	if name == "sequence_word_equations_relations.smt2" {
		// The fifth scope, (str.contains "abc" x) with |x| = 2, has two models,
		// "ab" and "bc". Which one is printed depends on search state carried
		// over from earlier scopes (z3 4.7.1 itself prints either, depending on
		// which earlier scopes ran). The Go seq theory does not yet follow
		// theory_seq's unit decomposition (a TRACE build shows z3 creating
		// (= "a" (seq.unit #x61)) atoms in the first scope that Go does not),
		// so its choice can differ. Accept exactly the valid models there.
		lines := strings.Split(output, "\n")
		models := 0
		for i, line := range lines {
			if strings.HasPrefix(line, "((x ") {
				models++
				if models == 4 && (line == "((x \"ab\"))" || line == "((x \"bc\"))") {
					lines[i] = "((x <length-2 substring of \"abc\">))"
				}
			}
		}
		output = strings.Join(lines, "\n")
	}
	if name == "fixedpoint_pdr_sliced_trace.smt2" || name == "fixedpoint_pdr_sliced_compound_tail.smt2" || name == "fixedpoint_pdr_sliced_compound_head.smt2" {
		// Native relation-query setup first allocates a synthetic query
		// predicate, while Go's public declaration-query surface does not.
		// The resulting slice fresh-ID offset is nonsemantic.
		output = strings.NewReplacer("P!slice!0", "P!slice!#", "P!slice!1", "P!slice!#").Replace(output)
	}
	if name == "fp_spacer_array_instantiation.smt2" {
		// dl_mk_array_instantiation.cpp writes its complete input/output rule
		// dumps to stdout unconditionally. Compare the actual command response
		// while retaining the native executable as the semantic oracle.
		var responses []string
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "sat" || line == "unsat" || line == "unknown" {
				responses = append(responses, line)
			}
		}
		return strings.Join(responses, "\n")
	}
	if name == "difference_shortcut_cores_real.smt2" || name == "difference_shortcut_cores_int.smt2" {
		// Conflict analysis stores core labels in an unordered declaration map.
		// Compare exact membership within each core, keeping every result/scope.
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")") {
				labels := strings.Fields(line[1 : len(line)-1])
				sort.Strings(labels)
				line = "(" + strings.Join(labels, " ") + ")"
			}
			lines[i] = line
		}
		return strings.Join(lines, "\n")
	}

	if name == "sat_cores.smt2" {
		// Source dep2asm is an unordered obj_map. This vector requires all
		// three labels, so compare the exact core membership, not hash order.
		fields := strings.Fields(strings.NewReplacer("(", " ", ")", " ").Replace(output))
		if len(fields) == 4 && fields[0] == "unsat" {
			sort.Strings(fields[1:])
			return strings.Join(fields, " ")
		}
	}
	return canonicalOutput(output)
}

// Keep this reference limitation separate from the agreement set. Direct smt
// inherits QF_LIA after pb2bv has introduced BV terms, so the release reference
// omits the required BV theory and can return an invalid integer model. The
// corresponding ALL vector checks the complete mixed-theory model in Go/C++;
// source QF_IDL's eventual strategy instead bit-blasts before its SAT backend.
func TestSMTLibRunnerPB2BVArithmeticLogicReferenceGap(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	script, err := os.ReadFile(filepath.Join("test_vectors", "pb2bv_qflia_direct_smt_reference_gap.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = bytes.NewReader(script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	want := "sat\n(((>= (+ (* 2 x) (* 3 y) (* 5 z) (* 7 w)) 17) false))\n"
	if canonicalOutput(string(output)) != canonicalOutput(want) {
		t.Fatalf("reference logic-profile behavior changed; reassess this isolated gap: %s", output)
	}
}

// The native combined solver uses its strategic tactic pipeline before the
// incremental core is selected. Preserve that nonincremental LIRA route while
// the matching incremental UTVPI variant intentionally remains unknown.
func TestSMTLibRunnerUTVPINonincrementalStrategy(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "utvpi_mixed_sum_model.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(bytes.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if canonicalOutput(output.String()) != "sat" {
		t.Fatal("ALL did not select the quantifier-free LIRA strategy", output.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = bytes.NewReader(script)
		native, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(native))
		}
		if canonicalOutput(string(native)) != "sat" {
			t.Fatal("native strategic-solver result changed", string(native))
		}
	}
}

// Preserve whitespace inside quoted names and strings; only printer layout
// outside token spellings may differ between the two executables.
func canonicalOutput(text string) string {
	var out strings.Builder
	var quoted byte
	escape, space := false, false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quoted != 0 {
			out.WriteByte(c)
			if quoted == '"' && c == '"' && i+1 < len(text) && text[i+1] == '"' {
				out.WriteByte(text[i+1])
				i++
				continue
			}
			if c == quoted && !escape {
				quoted = 0
			}
			escape = quoted == '|' && c == '\\'
			continue
		}
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			space = out.Len() != 0
			continue
		}
		if space {
			out.WriteByte(' ')
			space = false
		}
		out.WriteByte(c)
		if c == '"' || c == '|' {
			quoted = c
			escape = false
		}
	}
	return out.String()
}

func TestSMTLibRunnerProofScript(t *testing.T) {
	for _, name := range []string{"reset_proof_sessions.smt2", "proof.smt2", "assertion_conflict_proof.smt2", "check_phase_repeated_proof.smt2", "recursive_sum_proof.smt2", "datatype_constructor_conflict_proof.smt2", "string_literal_proof.smt2", "datatype_cycle_proof.smt2", "datatype_update_equality_proof.smt2", "datatype_metadata_proof.smt2", "closest_theory_variables_proof.smt2", "pb_native_watches_proof.smt2", "pb_simplex_proof.smt2"} {
		t.Run(name, func(t *testing.T) { testSMTLibProofScript(t, name) })
	}
}

func testSMTLibProofScript(t *testing.T, name string) {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("test_vectors", name))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "unsat\n") || !strings.Contains(out.String(), "(asserted") || !strings.Contains(out.String(), "false)") {
		t.Fatal("missing proof output", out.String())
	}
	if name == "reset_proof_sessions.smt2" && !completeResetProofSessions(out.String()) {
		t.Fatal("reset lost one of the two complete proof sessions", out.String())
	}
	if name == "string_literal_proof.smt2" && !strings.Contains(out.String(), `"quoted""text"`) {
		t.Fatal("proof lost its source quoted string literal", out.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = bytes.NewReader(script)
		value, err := cmd.CombinedOutput()
		if err != nil || !strings.HasPrefix(string(value), "unsat\n") || !strings.Contains(string(value), "(asserted") || !strings.Contains(string(value), "false)") {
			t.Fatal("C++ proof output", string(value), err)
		}
		if name == "reset_proof_sessions.smt2" && !completeResetProofSessions(string(value)) {
			t.Fatal("C++ reset lost one of the two complete proof sessions", string(value))
		}
		if name == "string_literal_proof.smt2" && !strings.Contains(string(value), `"quoted""text"`) {
			t.Fatal("C++ proof lost its source quoted string literal", string(value))
		}
	}
}

func completeResetProofSessions(output string) bool {
	sections := strings.Split(output, "unsat\n")
	if len(sections) != 3 || sections[0] != "" {
		return false
	}
	for _, proof := range sections[1:] {
		if !strings.Contains(proof, "(asserted") || !strings.Contains(proof, "false)") {
			return false
		}
	}
	return true
}

// The retained C++ backend gives up on the unbounded satisfiable scopes in
// proof mode. Preserve this case separately from strict output comparisons;
// x=p=q=0 is an independently checked rational witness for those scopes.
func TestSMTLibRunnerNestedUnboundedProofModeReferenceUnknown(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "nested_bound_dependency_scopes.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	unbounded := strings.Replace(string(script), "(assert (>= x 2.0))\n", "", 1)
	var out bytes.Buffer
	if err = run(strings.NewReader(unbounded), &out); err != nil {
		t.Fatal(err)
	}
	// With proofs, the SMT-LIB front end uses the plain SMT solver
	// (produce_interpolants is a synonym of produce_proofs), which answers
	// unknown here, as the C++ binary does.
	if canonicalOutput(out.String()) != canonicalOutput("unsat\nunknown\nunsat\nunknown\n") {
		t.Fatal(out.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		for _, entry := range []struct{ script, expected string }{
			{unbounded, "unsat\nunknown\nunsat\nunknown\n"},
			{strings.Replace(unbounded, "(push 1)", "(assert (= x 0.0))\n(push 1)", 1), "unsat\nsat\nunsat\nsat\n"},
		} {
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(entry.script)
			value, err := cmd.CombinedOutput()
			if err != nil || canonicalOutput(string(value)) != canonicalOutput(entry.expected) {
				t.Fatal("C++ unbounded scope or zero witness", string(value), err)
			}
		}
	}
}

func TestSMTLibRunnerCommandLine(t *testing.T) {
	for _, args := range [][]string{{"-in"}, {"-smt2", "test_vectors/uf_congruence.smt2"}, {"-file:test_vectors/uf_congruence.smt2"}, {"--", "test_vectors/uf_congruence.smt2"}} {
		var out bytes.Buffer
		if err := runArguments(args, strings.NewReader("(assert false)(check-sat)"), &out); err != nil {
			t.Fatal(args, err)
		}
		if out.String() != "unsat\n" {
			t.Fatal(args, out.String())
		}
	}
	for _, args := range [][]string{nil, {"-in", "test_vectors/uf_congruence.smt2"}} {
		var out bytes.Buffer
		if err := runArguments(args, strings.NewReader(""), &out); err == nil {
			t.Fatal("invalid frontend selection accepted", args)
		}
	}
	var out bytes.Buffer
	if err := runArguments([]string{"proof=true", "-in"}, strings.NewReader("(assert false)(check-sat)(get-proof)"), &out); err != nil {
		t.Fatal("source command-line parameter refresh", err)
	}
	if !strings.HasPrefix(out.String(), "unsat\n") || !strings.Contains(out.String(), "asserted") {
		t.Fatal(out.String())
	}

	const statisticsScript = "(assert false)(check-sat)"
	out.Reset()
	if err := runArguments([]string{"-st", "-in"}, strings.NewReader(statisticsScript), &out); err != nil {
		t.Fatal("statistics command line", err)
	}
	if !strings.HasPrefix(out.String(), "unsat\n(") || !strings.Contains(out.String(), ":time") || !strings.Contains(out.String(), ":total-time") || !strings.HasSuffix(out.String(), ")\n") {
		t.Fatal("statistics output", out.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-st", "-in")
		cmd.Stdin = strings.NewReader(statisticsScript)
		native, err := cmd.CombinedOutput()
		if err != nil || !strings.HasPrefix(string(native), "unsat\n(") || !strings.Contains(string(native), ":time") || !strings.Contains(string(native), ":total-time") {
			t.Fatal("native statistics output", err, string(native))
		}
	}
	out.Reset()
	if err := runArguments([]string{"-ist", "-in"}, strings.NewReader(statisticsScript), &out); err != nil || out.String() != "unsat\n" {
		t.Fatal("internal statistics must remain inert for SMT-LIB", err, out.String())
	}
}

// The retained C++ executable can return models violating p and (or a b)
// after changing relevancy from 0 to 2. Keep this case outside the strict
// comparison table: require valid Go models and expose the reference defect.
// The fork's backport of upstream z3 5dfe4a4b4 keeps relevancy from being
// raised after setup, so models stay valid (z3 4.7.1 printed p = false).
func TestSMTLibRunnerRelevancyUpdatesKeepModelsValid(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "relevancy_update_scopes.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	valid := "sat\n((p true) ((or a b) true))\nsat\n((p true) ((or a b) true))\nsat\n((p true) ((or a b) true))\nunsat\nsat\n((p true) ((or a b) true))\n"
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if canonicalOutput(out.String()) != canonicalOutput(valid) {
		t.Fatal("Go model violates an active assertion after option update or pop", out.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = bytes.NewReader(script)
		value, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal("C++ reference", err, string(value))
		}
		if canonicalOutput(string(value)) == canonicalOutput(valid) {
			return // A reference with the model defect fixed agrees with Go.
		}
		knownDefect := "sat\n((p true) ((or a b) true))\nsat\n((p false) ((or a b) false))\nsat\n((p false) ((or a b) false))\nunsat\nsat\n((p false) ((or a b) false))\n"
		if canonicalOutput(string(value)) != canonicalOutput(knownDefect) {
			t.Fatal("unexpected C++ option-update behavior", string(value))
		}
		t.Log("C++ reference returned models violating asserted formulas after the relevancy update; Go models remain valid")
	}
}
