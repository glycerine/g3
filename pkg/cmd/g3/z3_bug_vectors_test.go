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
	// datatype get_sort_size keys sparam::subst by the actual sort
	"z3_bug_datatype_param_size_subst.smt2": "unsat\n",

	// goal::update pushes the unflattened formula (FORK_BUGFIXES known bug)
	"z3_bug_goal_update_flattened_false.smt2": "(goals\n(goal\n  false\n  :precision precise :depth 1)\n)\n",

	"z3_bug_automaton_product.smt2":        "sat\n(model \n  (define-fun x () String\n    \"\")\n)\n",
	"z3_bug_qe_nested_store.smt2":          "sat\n",
	"z3_bug_opt_stale_objective_term.smt2": "sat\n(objectives\n (x 7)\n (y oo)\n (q (- 3))\n)\n",

	// muz_pdr_01
	"z3_bug_pdr_boogie_without_proof_trace.smt2": "sat\n(derivation\n(step\n s!0 (inv 0)\n null\n (subst)\n (labels)\n (ref)\n)\n(step\n s!1 (inv 1)\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!0)\n)\n(step\n s!2 (inv 2)\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!1)\n)\n(step\n s!3 query!0\n null\n (subst\n  (= B 0)\n  (= A 1))\n (labels)\n (ref s!2)\n)\n)\n\n", // muz_pdr_02-F2

	// duality_03-F2

	// muz_fp_03 (duality quantify_arrays rules not restored)

	// duality_01-F4 (certificate substitutions of qe-lite-eliminated
	// variables). The nullary steps print as `(err)` (FUZZ_F-F6).
	"z3_bug_duality_certificate_eliminated_var.smt2": "sat\n(derivation\n(step s!4 (q0 2)\n rule!1  (subst\n    (= x 2)\n  )\n  (labels  )\n  (ref   ))\n(step s!3 (q0 5)\n rule!2  (subst\n    (= x 2)\n    (= x1 5)\n  )\n  (labels  )\n  (ref  s!4  ))\n(step s!2 (err)\n rule!3  (subst\n    (= x 5)\n  )\n  (labels  )\n  (ref  s!3  ))\n(step s!1 (@Fail!0)\n rule!4  (subst\n  )\n  (labels  )\n  (ref  s!2  ))\n)\n(model \n\"@under2 -> false\n@under4 -> false\n@b_2 -> true\n@b_3 -> true\n@b_1 -> true\n@under3 -> false\n@b_4 -> true\n\")\n\n",

	// duality_01-F6 (Build interpolates the leaf-assumption proof)

	// duality_02-F2 (FixCurrentState fixes the @under flags)

	// duality_01-F3 follow-up (query after a failed Duality query)

	// muz_rel_04-F2

	// default_table_checked nullary check_table (no AUDIT.md finding)

	// muz_small_01-F8
	// qlinear::compile asserts per formula (muz_small_01-F7), which steers
	// the search to a shorter trace.

	// muz_small_01-F9
	// The compiled rules go to os.Stdout; TestZ3BugDDNFExtractWidthDump
	// checks them.
	"z3_bug_ddnf_extract_width.smt2": "unknown\ndone\n",

	// util_misc_01-F1

	// muz_transforms_01-F6
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
