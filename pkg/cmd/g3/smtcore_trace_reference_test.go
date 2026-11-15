package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// smtcoreTraceVectors compare the trace=true AST log of g3 and the C++
// reference. Fingerprint addresses ([new-match] 0x...) are masked.
var smtcoreTraceVectors = []string{
	"array_default_store_epsilon_order.smt2", // AUDIT.md smtcore_09-F3
	"array_interface_eq_var_owner.smt2",      // AUDIT.md smtcore_09-F4
}

var smtcoreTracePointer = regexp.MustCompile(`0x[0-9a-f]+`)

func TestSMTCoreTraceLogCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the C++ z3 executable")
	}
	for _, name := range smtcoreTraceVectors {
		t.Run(name, func(t *testing.T) {
			input, err := filepath.Abs(filepath.Join("test_vectors", name))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			goLog, cppLog := filepath.Join(dir, "g3.log"), filepath.Join(dir, "z3.log")
			var goOutput bytes.Buffer
			if err = runArguments([]string{"trace=true", "trace_file_name=" + goLog, input}, strings.NewReader(""), &goOutput); err != nil {
				t.Fatal(err, goOutput.String())
			}
			cppOutput, err := exec.Command(reference, "trace=true", "trace_file_name="+cppLog, input).CombinedOutput()
			if err != nil {
				t.Fatal("C++ reference", err, string(cppOutput))
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
			got, want = smtcoreTracePointer.ReplaceAll(got, []byte("0x")), smtcoreTracePointer.ReplaceAll(want, []byte("0x"))
			if !bytes.Equal(got, want) {
				t.Fatalf("Go/C++ trace log mismatch\nGo:\n%s\nC++:\n%s", got, want)
			}
		})
	}
}
