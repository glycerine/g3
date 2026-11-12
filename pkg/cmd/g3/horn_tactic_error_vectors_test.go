package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hornTacticErrorScripts print the source rule_properties default_exception
// text from horn and horn-simplify: apply reports it as a parser error with
// the command position, while check-sat-using catches it as "tactic failed"
// without trimming the rule display's trailing newline. run returns the
// first reported parser error.
var hornTacticErrorScripts = map[string]string{
	"horn_tactic_nested_recursive_error.smt2":      "(error \"line 11 column 11: recursive predicate (let ((a!1 (= c (or a (> (+ x y) 4)))))\n  (not a!1)) occurs nested in the body of a rule\")\n(error \"tactic failed: recursive predicate (let ((a!1 (= c (or a (> (+ x y) 4)))))\n  (not a!1)) occurs nested in the body of a rule\")\n(error \"line 13 column 20: recursive predicate (let ((a!1 (= c (or a (> (+ x y) 4)))))\n  (not a!1)) occurs nested in the body of a rule\")\n(error \"tactic failed: recursive predicate (let ((a!1 (= c (or a (> (+ x y) 4)))))\n  (not a!1)) occurs nested in the body of a rule\")\n",
	"horn_tactic_uninterpreted_error.smt2":         "(error \"line 9 column 11: Uninterpreted 'z' in <null>:\nquery!0() :- \n (not (= (f z) (+ z 1))).\")\n(error \"tactic failed: Uninterpreted 'z' in <null>:\nquery!1() :- \n (not (= (f z) (+ z 1))).\n\")\n(error \"line 11 column 20: Uninterpreted 'z' in <null>:\nquery!2() :- \n (not (= (f z) (+ z 1))).\")\n(error \"tactic failed: Uninterpreted 'z' in <null>:\nquery!3() :- \n (not (= (f z) (+ z 1))).\n\")\n",
	"horn_tactic_datalog_uninterpreted_error.smt2": "(error \"line 8 column 11: Uninterpreted 'v' in <null>:\np() :- \n (not (bvsgt v #x10)).\")\n(error \"tactic failed: Uninterpreted 'v' in <null>:\np() :- \n (not (bvsgt v #x10)).\n\")\n(error \"line 10 column 20: Uninterpreted 'v' in <null>:\np() :- \n (not (bvsgt v #x10)).\")\n(error \"tactic failed: Uninterpreted 'v' in <null>:\np() :- \n (not (bvsgt v #x10)).\n\")\n",
}

func TestSMTLibRunnerHornTacticErrors(t *testing.T) {
	for name, expected := range hornTacticErrorScripts {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err = run(bytes.NewReader(script), &out)
			first := strings.SplitN(expected, "\")\n", 2)[0]
			if err == nil || "(error \""+strings.TrimRight(err.Error(), "\n") != first {
				t.Fatalf("expected the first parser error %q, got %v", first, err)
			}
			if out.String() != expected {
				t.Fatalf("Go SMT-LIB result %q want %q", out.String(), expected)
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				cmd := exec.Command(reference, "-in")
				cmd.Stdin = bytes.NewReader(script)
				var native bytes.Buffer
				cmd.Stdout = &native
				if err = cmd.Run(); err == nil {
					t.Fatal("C++ reference should report the tactic errors")
				}
				if native.String() != expected {
					t.Fatalf("C++ result %q want %q", native.String(), expected)
				}
			}
		})
	}
}
