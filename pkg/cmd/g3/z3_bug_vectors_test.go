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
