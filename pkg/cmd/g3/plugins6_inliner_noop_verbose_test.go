//go:build g3trace

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins6InlinerNoopVerbose: AUDIT.md FUZZ-F5. When the only thing
// mk_rule_inliner would do is drop the rules of an inlinable predicate, it
// reports no-op and keeps the source rules (transform_rules'
// something_done), so the program still loads and stores r; the expected
// lines are the fixed fork's -v:2 stderr.
func TestPlugins6InlinerNoopVerbose(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	log := runVerboseProgressChild(t, []string{"-v:2", filepath.Join(wd, "test_vectors", "plugins6_inliner_noop.smt2")})
	if !strings.Contains(log, "(transform N7datalog15mk_rule_inlinerE...no-op ") {
		t.Fatalf("inliner not a no-op:\n%s", log)
	}
	want := []string{
		"load r into 0",
		"mk_unary_singleton into 1 sort:Bool val:1",
		"union 1 into 0",
		"mark_saturated r",
		"store 0 into r",
	}
	var got []string
	for _, line := range strings.Split(log, "\n") {
		if line != "" && !strings.HasPrefix(line, "(") && !strings.Contains(line, "...") {
			got = append(got, line)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("-v:2 lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
