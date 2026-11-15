package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// AUDIT.md muz_fp_03-F8: rel_context::saturate bounds each round by
// soft_timeout() (dl_context's smt_params m_timeout, i.e. smt.timeout as set
// on the command line) and reports unknown / timeout when it expires.
func TestDatalogSoftTimeoutFromCommandLine(t *testing.T) {
	path := filepath.Join("test_vectors", "datalog_soft_timeout.smt2")
	const want = "unknown\ntimeout\n"
	var out bytes.Buffer
	if err := runArguments([]string{"smt.timeout=50", path}, bytes.NewReader(nil), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		native, err := exec.Command(reference, "smt.timeout=50", path).Output()
		if err != nil {
			t.Fatal("C++ reference", err, string(native))
		}
		if string(native) != want {
			t.Fatalf("C++ reference %q, want %q", native, want)
		}
	}
}
