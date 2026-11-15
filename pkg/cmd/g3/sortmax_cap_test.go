//go:build !g3debug

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// sortmaxAboveOldCapResult is build/z3's output for
// sortmax_above_old_cap.smt2 (identical with and without fork fix #A).
const sortmaxAboveOldCapResult = "sat\n(objectives\n ( 3)\n)\n(model \n  (define-fun z () Bool\n    true)\n  (define-fun y () Bool\n    false)\n  (define-fun x () Bool\n    false)\n)\n"

// AUDIT.md opt_01-F1: maxsat_engine=sortmax builds sortmax whatever the
// weight total. The vector expands to 4098 network inputs (about 700000
// clauses), above the 4096 at which g3 used to switch to MaxRes. It takes
// seconds in a release build but minutes with g3debug's checks, so it is
// kept out of scriptResults and of debug builds. With G3_CPP_REFERENCE set
// it is also compared with the C++ output.
func TestSortMaxAboveOldInputCap(t *testing.T) {
	name := "sortmax_above_old_cap.smt2"
	script, err := os.ReadFile(filepath.Join("test_vectors", name))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(bytes.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if out.String() != sortmaxAboveOldCapResult {
		t.Fatalf("g3 output\n%s\nwant\n%s", out.String(), sortmaxAboveOldCapResult)
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		return
	}
	cmd := exec.Command(reference, "-in")
	cmd.Stdin = bytes.NewReader(script)
	cppOutput, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(cppOutput) != out.String() {
		t.Fatalf("C++ output\n%s\ng3 output\n%s", cppOutput, out.String())
	}
}
