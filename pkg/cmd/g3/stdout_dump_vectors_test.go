package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// stdoutDumpResults holds vectors whose transforms write rule dumps straight
// to std::cout (dl_mk_array_instantiation.cpp), so the expected text is the
// complete z3 stdout: the process-stdout dump followed by the command output.
var stdoutDumpResults = map[string]string{
	// AUDIT.md muz_transforms_01-F2: retrieve_all_selects follows the
	// union-find next cycle from the array (p!inst instance order).
	"array_inst_eqclass_order.smt2": "Array Instantiation called with parameters : enforce=0 nb_quantifier=1 slice_technique=no-slicing\nInput rules = \n; rule count: 3\n; predicate count: 3\n; output: query!0\n<null>:\nq() :- \n p(#2),\n p(#4),\n (not (<= (select (store (:var 2) (:var 1) 1) (:var 0)) 5)),\n (not (<= (select (:var 2) (:var 3)) 0)),\n (= (:var 4) (store (:var 2) (:var 1) 1)).\n<null>:\np(#0) :- \n (= (select (:var 0) 0) 0).\n<null>:\nquery!0() :- \n q().\n\n\nOutput rules = \n; rule count: 3\n; predicate count: 3\n; output: query!0!inst\n<null>:\nq!inst() :- \n p!inst(#5,#2,#3),\n p!inst(#6,#2,#0),\n p!inst(#7,#4,#0),\n p!inst(#8,#4,#3),\n (= (:var 6) (select (:var 2) (:var 0))),\n (= (:var 5) (select (:var 2) (:var 3))),\n (= (:var 7) (select (:var 4) (:var 0))),\n (= (:var 4) (store (:var 2) (:var 1) 1)),\n (not (<= (select (:var 2) (:var 3)) 0)),\n (not (<= (select (store (:var 2) (:var 1) 1) (:var 0)) 5)),\n (= (:var 8) (select (:var 4) (:var 3))).\n<null>:\np!inst(#2,#0,#1) :- \n (= (select (:var 0) 0) 0),\n (= (:var 2) (select (:var 0) (:var 1))).\n<null>:\nquery!0!inst() :- \n q!inst().\nsat\n",
	// AUDIT.md muz_transforms_01-F1: normalizer_cfg BR_REWRITE1 re-sorts
	// after detect_equivalences (the second q rule's equality order).
	"interp_tail_equiv_resort.smt2": "Array Instantiation called with parameters : enforce=0 nb_quantifier=1 slice_technique=no-slicing\nInput rules = \n; rule count: 4\n; predicate count: 3\n; output: query!0\n<null>:\np(0,0,0,0).\n<null>:\nq() :- \n p(#0,#3,#2,#1),\n (= (not (<= (:var 3) 0)) (<= (:var 2) 0)),\n (or (<= (:var 0) 1) (<= (:var 1) 0)),\n (<= (:var 0) 0).\n<null>:\nq() :- \n p(#3,#2,#0,#1),\n (= (<= (:var 2) 0) (not (<= (:var 3) 0))),\n (= (<= (:var 1) 0) (<= (:var 0) 0)),\n (not (<= (:var 0) 3)).\n<null>:\nquery!0() :- \n q().\n\n\nOutput rules = \n; rule count: 4\n; predicate count: 3\n; output: query!0!inst\n<null>:\np!inst(0,0,0,0).\n<null>:\nq!inst() :- \n p!inst(#0,#3,#2,#1),\n (or (<= (:var 0) 1) (<= (:var 1) 0)),\n (= (not (<= (:var 3) 0)) (<= (:var 2) 0)),\n (<= (:var 0) 0).\n<null>:\nq!inst() :- \n p!inst(#3,#2,#0,#1),\n (= (<= (:var 1) 0) (<= (:var 0) 0)),\n (= (<= (:var 2) 0) (not (<= (:var 3) 0))),\n (not (<= (:var 0) 3)).\n<null>:\nquery!0!inst() :- \n q!inst().\nunsat\n",
}

func runCapturingStdout(t *testing.T, script []byte) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = writer
	captured := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		captured <- data
	}()
	var out bytes.Buffer
	runErr := run(bytes.NewReader(script), &out)
	os.Stdout = saved
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	dump := <-captured
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatal(runErr, string(dump), out.String())
	}
	return string(dump) + out.String()
}

func TestStdoutDumpVectors(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	for name, expected := range stdoutDumpResults {
		t.Run(name, func(t *testing.T) {
			script, err := os.ReadFile(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			if got := runCapturingStdout(t, script); got != expected {
				t.Fatalf("Go output\n%s\nwant\n%s", got, expected)
			}
			if reference == "" {
				return
			}
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = bytes.NewReader(script)
			native, err := cmd.Output()
			if err != nil {
				t.Fatal("C++ reference", err, string(native))
			}
			if string(native) != expected {
				t.Fatalf("C++ reference output\n%s\nwant\n%s", native, expected)
			}
		})
	}
}
