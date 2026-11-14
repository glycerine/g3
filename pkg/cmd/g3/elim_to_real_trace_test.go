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
	// z3 4.7.1: the converted Int numeral of the failed elim_to_real is
	// gone before (<= y2 y1) is built, so that application reuses #28.
	checkTraceIdentity(t, `(declare-const y1 Real)
(declare-const y2 Real)
(simplify (<= (- 1.0) y1) :elim_to_real true)
(simplify (<= y2 y1))
`, "(<= (- 1.0) y1)\n(<= y2 y1)\n", "\n[mk-app] #28 <= #27 #25\n")
}

// AUDIT.md FUZZ_A-F9: display_assignment builds both to_expr temporaries
// of an arithmetic interval and keeps them to the end of the statement.
func TestObjectiveIntervalDisplayTraceIdentity(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("test_vectors", "opt_objectives_interval_before_unbounded.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	checkTraceIdentity(t, string(script), scriptResults["opt_objectives_interval_before_unbounded.smt2"],
		"\n[mk-app] #50 epsilon\n[mk-app] #53 * #34 #50\n[mk-app] #55 + #34 #53\n")
}

// checkTraceIdentity runs script with trace=true, checks its output and
// that the trace log contains want, and with G3_CPP_REFERENCE compares the
// whole log with the C++ one.
func checkTraceIdentity(t *testing.T, script, output, want string) {
	t.Helper()
	dir := t.TempDir()
	traceScript := func(log string) string {
		return fmt.Sprintf("(set-option :trace true)\n(set-option :trace_file_name %q)\n", log) + script
	}
	goLog := filepath.Join(dir, "go.log")
	var actual bytes.Buffer
	if err := run(strings.NewReader(traceScript(goLog)), &actual); err != nil {
		t.Fatal(err, actual.String())
	}
	if actual.String() != output {
		t.Fatal(actual.String())
	}
	goTrace, err := os.ReadFile(goLog)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(goTrace, []byte(want)) {
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
