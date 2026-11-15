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
	// Datatype instances after pop (upstream 19f655c69 and the fork) and an
	// alias used in a popped scope (still present upstream; fork fixed).
	"z3_bug_datatype_inst_after_pop.smt2":     "sat\nsat\n",
	"z3_bug_define_sort_alias_after_pop.smt2": "sat\n",
	// get-value reports the evaluator's max-steps exception (upstream 335d672bf).
	"z3_bug_get_value_max_steps.smt2": "sat\n((error \"line 7 column 38: max. steps exceeded\")\n((x 1))\n(((+ x y 1 (* 2 x) (* 3 y)) 0))\n",
	// bvarray2uf reports quantified goals as "not handled" (upstream 4bad2dd92).
	"z3_bug_bvarray2uf_quantifier.smt2": "(error \"line 5 column 17: not handled by bvarray2uf\")\n(error \"tactic failed: not handled by bvarray2uf\")\n(error \"line 9 column 17: not handled by bvarray2uf\")\n",
	// symmetry-reduce rejects a quantified goal (upstream 330b3cc8d).
	"z3_bug_symmetry_reduce_quantified.smt2": "(error \"tactic failed: symmetry_reduce does not apply to quantified goals\")\nsat\nunknown\n",
	// sine-filter on an empty goal and on a last formula without symbols.
	"z3_bug_sine_filter_component.smt2": "(goals\n(goal\n  :precision over :depth 1)\n)\n(goals\n(goal\n  (> x 0)\n  :precision over :depth 1)\n)\n",
	// elim-small-bv enumerates every value of a small bit-vector variable
	// and leaves a quantifier that does not fit in max_steps unchanged.
	"z3_bug_elim_small_bv.smt2":               "(goals\n(goal\n  (forall ((z Int)) (<= x z))\n  :precision precise :depth 1)\n)\n(goals\n(goal\n  false\n  :precision precise :depth 1)\n)\nunsat\n(goals\n(goal\n  (forall ((y (_ BitVec 2))) (q y))\n  :precision precise :depth 1)\n)\n(goals\n(goal\n  (q #b00)\n  (q #b01)\n  (q #b10)\n  (q #b11)\n  :precision precise :depth 1)\n)\n",
	"elim_small_bv_max_steps_simplifier.smt2": "(goals\n(goal\n  (forall ((x (_ BitVec 2)) (y (_ BitVec 2)) (z (_ BitVec 2))) (p x y z))\n  (forall ((x (_ BitVec 2))) (q x))\n  :precision precise :depth 1)\n)\n",
	// muz_pdr_01

}

func TestZ3BugVectorsKeepCorrectAnswers(t *testing.T) {
	for name, expected := range z3BugResults {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil && !z3BugExpectErrors[name] {
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
