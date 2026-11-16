package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
)

// TestPlugins6Rlimit: AUDIT.md FUZZ-F5. The rlimit-count of the plugins6
// vectors equals the fixed fork's.
func TestPlugins6Rlimit(t *testing.T) {
	rlimit := regexp.MustCompile(`:rlimit-count +([0-9]+)`)
	for name, want := range map[string]string{
		"plugins6_inliner_noop.smt2":              "61",
		"plugins6_external_relation_missing.smt2": "182",
	} {
		var out, diagnostics bytes.Buffer
		args := []string{"-st", filepath.Join("test_vectors", name)}
		if err := runArgumentsWithDiagnostics(args, bytes.NewReader(nil), &out, &diagnostics); err != nil && !expectedParseError(name, err) {
			t.Fatal(name, err, out.String())
		}
		got := rlimit.FindStringSubmatch(out.String())
		if got == nil || got[1] != want {
			t.Fatalf("%s: rlimit-count %v, want %s\n%s", name, got, want, out.String())
		}
	}
}
