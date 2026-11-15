package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// z3BugResults holds the correct g3 output for each vector that exposes a
// z3 4.7.1 bug g3 deliberately does not reproduce (see
// test_vectors/z3_bug_exclusions.txt). These vectors are kept out of
// scriptResults so the C++ reference comparison does not run them; this
// test locks in g3's correct answers instead.
var z3BugResults = map[string]string{
	"z3_bug_forward_trig_unsafe_after_purify.smt2":    "unknown\n((x 10.0)\n (y 10.0))\n", // tactic_arith_02-F2
	"z3_bug_eq2bv_stale_rewriter_cache.smt2":          "unsat\n",                          // tactic_arith_01-F3
	"z3_bug_pb2bv_numeral_lhs.smt2":                   "unsat\nunsat\n",                   // tactic_arith_03-F1
	"z3_bug_automaton_product.smt2":                   "sat\n(model \n  (define-fun x () String\n    \"\")\n)\n",
	"z3_bug_qe_nested_store.smt2":                     "sat\n",
	"z3_bug_opt_stale_objective_term.smt2":            "sat\n(objectives\n (x 7)\n (y oo)\n (q (- 3))\n)\n",
	"z3_bug_dom_simplify_root_resimplify.smt2":        "sat\nsat\n(goals\n(goal\n  (= true (ite p1 p0 false))\n  :precision precise :depth 1)\n)\n",
	"z3_bug_dom_simplify_oscillation.smt2":            "unsat\nunsat\n(goals\n(goal\n  (not p3)\n  (not (and p1 true false))\n  (not true)\n  :precision precise :depth 1)\n)\n",
	"z3_bug_elim_term_ite_stale_cache.smt2":           "sat\nunsat\n",
	"z3_bug_macro_finder_disabled_stale_macro.smt2":   "sat\nunsat\n",
	"z3_bug_pb_preprocess_ge_index.smt2":              "unsat\nunsat\n(goals\n(goal\n  false\n  :precision precise :depth 1)\n)\n",
	"z3_bug_pb_preprocess_resolve_min_index.smt2":     "unsat\nunsat\n",
	"z3_bug_pb_preprocess_resolve_no_complement.smt2": "unsat\nunsat\n",
	"z3_bug_update_field_constructor.smt2":            "unsat\n(mkp j red)\n", // smtcore_10-F15
	"z3_bug_dt_recognizer_relevancy1.smt2":            "sat\n((b false))\n",   // smtcore_10-F3

	// muz_pdr_01
	"z3_bug_pdr_nullary_cache.smt2":     "unsat\n",
	"z3_bug_pdr_nullary_cache_sat.smt2": "sat\n(and G B (D 3) (D 2) (D 1) (D 0) A (C 3) (C 2) (C 1) (C 0))\n",

	// duality_03-F2
	"z3_bug_duality_unhoist_repeated_child.smt2": "sat\n",

	// muz_rel_04-F2
	"z3_bug_lazy_sparse_nullary_union.smt2": "sat\ntrue\n",

	// muz_small_01-F8
	"z3_bug_qbmc_eval_level.smt2": "sat\n(let ((a!1 (forall ((A Int)) (=> (and (p A) (not (<= A 2))) q)))\n      (a!2 (forall ((A Int) (B Int)) (=> (and (p A) (= B (+ 1 A))) (p B)))))\n(let ((a!3 (hyper-res (hyper-res (asserted (=> q query!0))\n                                 (asserted a!1)\n                                 (=> (p 5) query!0))\n                      (asserted a!2)\n                      (=> (p 4) query!0))))\n(let ((a!4 (hyper-res (hyper-res a!3 (asserted a!2) (=> (p 3) query!0))\n                      (asserted a!2)\n                      (=> (p 2) query!0))))\n(let ((a!5 (hyper-res (hyper-res a!4 (asserted a!2) (=> (p 1) query!0))\n                      (asserted a!2)\n                      (=> (p 0) query!0))))\n  ((_ hyper-res 0 1) a!5 (asserted (p 0)) query!0)))))\n",

	// muz_small_01-F9
	// The compiled rules go to os.Stdout; TestZ3BugDDNFExtractWidthDump
	// checks them.
	"z3_bug_ddnf_extract_width.smt2": "unknown\ndone\n",

	// util_misc_01-F1
	"z3_bug_spacer_min_cut_shared_node.smt2":       "sat\n\n",
	"z3_bug_spacer_min_cut_shared_node_unsat.smt2": "unsat\n\n",

	// muz_transforms_01-F6
	"z3_bug_qi_weakening_proof.smt2": "sat\ntrue\n",
}

func TestZ3BugVectorsKeepCorrectAnswers(t *testing.T) {
	for name, expected := range z3BugResults {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if out.String() != expected {
				t.Fatalf("g3 output\n%s\nwant\n%s", out.String(), expected)
			}
		})
	}
}

// Every vector named in z3_bug_exclusions.txt with a z3_bug_ prefix must
// have a locked-in answer here.
func TestZ3BugExclusionsHaveResults(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("test_vectors", "z3_bug_exclusions.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(text), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "z3_bug_") {
			if _, ok := z3BugResults[line+".smt2"]; !ok {
				t.Error("no z3BugResults entry for", line)
			}
		}
	}
}

// muz_small_01-F9: the ddnf engine dumps its compiled rules to os.Stdout, as
// the C++ dump_rules writes to std::cout. z3 4.7.1 segfaults on this input.
func TestZ3BugDDNFExtractWidthDump(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "z3_bug_ddnf_extract_width.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	dump, err := os.CreateTemp(t.TempDir(), "ddnf-dump")
	if err != nil {
		t.Fatal(err)
	}
	defer dump.Close()
	saved := os.Stdout
	os.Stdout = dump
	var out bytes.Buffer
	err = run(bytes.NewReader(script), &out)
	os.Stdout = saved
	if err != nil {
		t.Fatal(err, out.String())
	}
	dumped, err := os.ReadFile(dump.Name())
	if err != nil {
		t.Fatal(err)
	}
	want := "(declare-rel q ())\n" +
		"(declare-rel p ((_ BitVec 2)))\n" +
		"(declare-var A (_ BitVec 2))\n" +
		"(rule (=> (= A #b01) (p A)))\n" +
		"(rule (=> (and (p A) (or (= A #b01) (= A #b10))) q))\n" +
		"(query q)\n"
	if string(dumped) != want {
		t.Fatalf("ddnf dump\n%s\nwant\n%s", dumped, want)
	}
	if out.String() != "unknown\ndone\n" {
		t.Fatalf("g3 output\n%s", out.String())
	}
}
