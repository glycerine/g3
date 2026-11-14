package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// AUDIT.md FUZZ_J-F3: execute_min_max takes optsmt::m_model, which a farkas
// run without a satisfiable midpoint never sets, so get-model has no model.
func TestOptimizationFarkasLexLeavesNoModel(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "opt_farkas_no_midpoint.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	script = append(script, "(get-model)\n"...)
	want := scriptResults["opt_farkas_no_midpoint.smt2"] + "(error \"line 12 column 10: model is not available\")\n"
	var output bytes.Buffer
	err = run(bytes.NewReader(script), &output)
	if err == nil || err.Error() != "line 12 column 10: model is not available" {
		t.Fatal("expected the get-model error", err)
	}
	if output.String() != want {
		t.Fatalf("output %q", output.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = bytes.NewReader(script)
		native, _ := cmd.Output()
		if string(native) != want {
			t.Fatalf("C++ output %q", native)
		}
	}
}
