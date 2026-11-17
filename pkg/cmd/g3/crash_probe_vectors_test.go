package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AUDIT.md FUZZ-F5 (relation plugins): the relation-plugin probes on which
// z3 4.7.1 crashes: 85 segfault (p_E_*, m_E_*, m_H_*: an Int/Bool relation
// under negation, sieve and product relations of every plugin, with Karr,
// explanations and relation-level explanations) and 2 abort with
// std::bad_cast (p_E_chk_bound_relation, p_E_chk_interval_relation). The
// fork fixes them (plugins2: product kind spec, sieve kinds and negation
// columns; plugins4: check_relation mixed joins, table_relation of an
// unhandled signature, table complement of several columns). Each .out is
// the fixed fork's standard output for the .smt2 next to it.
const crashProbeDirectory = "test_vectors/crash_probes_4_7_1"

func crashProbeNames(t *testing.T) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(crashProbeDirectory, "*.smt2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 87 {
		t.Fatalf("%d crash probes, want 87", len(names))
	}
	return names
}

func crashProbeWant(t *testing.T, name string) string {
	t.Helper()
	want, err := os.ReadFile(strings.TrimSuffix(name, ".smt2") + ".out")
	if err != nil {
		t.Fatal(err)
	}
	return string(want)
}

func TestCrashProbeVectors(t *testing.T) {
	for _, name := range crashProbeNames(t) {
		t.Run(filepath.Base(name), func(t *testing.T) {
			script, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if want := crashProbeWant(t, name); out.String() != want {
				t.Fatalf("g3:\n%s\nfixed fork:\n%s", out.String(), want)
			}
		})
	}
}

func TestCrashProbeVectorsCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to the C++ z3 executable")
	}
	for _, name := range crashProbeNames(t) {
		t.Run(filepath.Base(name), func(t *testing.T) {
			cmd := exec.Command(reference, filepath.Base(name))
			cmd.Dir = crashProbeDirectory
			out, err := cmd.Output()
			if err != nil {
				t.Fatal("C++ reference", err, string(out))
			}
			if want := crashProbeWant(t, name); string(out) != want {
				t.Fatalf("C++:\n%s\nwant:\n%s", out, want)
			}
		})
	}
}
