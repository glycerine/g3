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
	"z3_bug_pdr_boogie_without_proof_trace.smt2": "sat\n(derivation\n(step\n s!0 (inv 0)\n null\n (subst)\n (labels)\n (ref)\n)\n(step\n s!1 (inv 1)\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!0)\n)\n(step\n s!2 (inv 2)\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!1)\n)\n(step\n s!3 query!0\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!2)\n)\n)\n\n", // muz_pdr_02-F2

	// muz_small_01-F9
	// The compiled rules go to os.Stdout; TestZ3BugDDNFExtractWidthDump
	// checks them.
	"z3_bug_ddnf_extract_width.smt2": "unknown\ndone\n",
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
