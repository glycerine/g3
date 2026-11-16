package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
)

// TestPlugins5RouteRlimit: AUDIT.md FUZZ-F5 (relation plugins). Programs
// that now run on the relation plugins count the fixed fork's
// rlimit-count.
func TestPlugins5RouteRlimit(t *testing.T) {
	rlimit := regexp.MustCompile(`:rlimit-count +([0-9]+)`)
	for name, want := range map[string]string{
		"plugins5_bound_tr_sparse.smt2":        "60",
		"plugins5_nullary_tr_sparse.smt2":      "53",
		"plugins5_wide_tr_bitvector.smt2":      "53",
		"plugins5_tr_hashtable_favourite.smt2": "92",
		"plugins5_tr_bitvector_favourite.smt2": "848",
	} {
		var out, diagnostics bytes.Buffer
		args := []string{"-st", filepath.Join("test_vectors", name)}
		if err := runArgumentsWithDiagnostics(args, bytes.NewReader(nil), &out, &diagnostics); err != nil {
			t.Fatal(name, err, out.String())
		}
		got := rlimit.FindStringSubmatch(out.String())
		if got == nil || got[1] != want {
			t.Fatalf("%s: rlimit-count %v, want %s\n%s", name, got, want, out.String())
		}
	}
}
