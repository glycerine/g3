package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AUDIT.md FUZZ_A-F8: mk_le_ge_eq_core keeps new_new_arg1 (the converted
// left side) alive when elim_to_real fails on the right side, and
// elim_to_real_mon/pol release their partially converted factors. The
// lifetime decides which AST ids later terms get.
func TestElimToRealFailureTraceIdentity(t *testing.T) {
	dir := t.TempDir()
	traceScript := func(log string) string {
		return fmt.Sprintf(`(set-option :trace true)
(set-option :trace_file_name %q)
(declare-const y1 Real)
(declare-const y2 Real)
(simplify (<= (- 1.0) y1) :elim_to_real true)
(simplify (<= y2 y1))
`, log)
	}
	goLog := filepath.Join(dir, "go.log")
	var output bytes.Buffer
	if err := run(strings.NewReader(traceScript(goLog)), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "(<= (- 1.0) y1)\n(<= y2 y1)\n" {
		t.Fatal(output.String())
	}
	goTrace, err := os.ReadFile(goLog)
	if err != nil {
		t.Fatal(err)
	}
	// z3 4.7.1: the converted Int numeral of the failed elim_to_real is
	// gone before (<= y2 y1) is built, so that application reuses #28.
	if !bytes.Contains(goTrace, []byte("\n[mk-app] #28 <= #27 #25\n")) {
		t.Fatalf("unexpected AST ids:\n%s", goTrace)
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		cppLog := filepath.Join(dir, "cpp.log")
		cmd := exec.Command(reference, "-in")
		cmd.Stdin = strings.NewReader(traceScript(cppLog))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
		cppTrace, err := os.ReadFile(cppLog)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(goTrace, cppTrace) {
			t.Fatalf("Go/C++ trace mismatch\nGo:\n%s\nC++:\n%s", goTrace, cppTrace)
		}
	}
}
