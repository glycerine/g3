package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTraceLogLifetimesCPPReference compares the trace=true AST log of g3
// and the C++ reference byte for byte, like TestTraceLogCPPReference, for
// vectors whose standard output may hold (error ...) lines: the run may
// report a parse error as long as both binaries print the same output.
func TestTraceLogLifetimesCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the C++ z3 executable")
	}
	for _, name := range []string{
		// AUDIT.md FUZZ_C-F6: a failed expression keeps the parser's
		// expression stack until reset().
		"parse_error_expr_stack_ids.smt2",
	} {
		t.Run(name, func(t *testing.T) {
			input, err := filepath.Abs(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			goLog, cppLog := filepath.Join(dir, "g3.log"), filepath.Join(dir, "z3.log")
			var goOutput bytes.Buffer
			goErr := runArguments([]string{"trace=true", "trace_file_name=" + goLog, input}, strings.NewReader(""), &goOutput)
			cppOutput, cppErr := exec.Command(reference, "trace=true", "trace_file_name="+cppLog, input).Output()
			if (goErr == nil) != (cppErr == nil) {
				t.Fatalf("Go error %v, C++ error %v", goErr, cppErr)
			}
			if goOutput.String() != string(cppOutput) {
				t.Fatalf("Go/C++ output mismatch\nGo:\n%s\nC++:\n%s", goOutput.String(), cppOutput)
			}
			got, err := os.ReadFile(goLog)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(cppLog)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("Go/C++ trace log mismatch\nGo:\n%s\nC++:\n%s", got, want)
			}
		})
	}
}
