package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins15SweepOutputs: datalog programs of the plugins15 sweep
// (recursion shapes under default_relation, default_table, check_relation,
// generate_explanations, magic sets and xform settings) whose answers
// differed from the fixed fork's (plugins15_sweep_<name>.stdout): udoc's
// filter_equal functor reused by a cached select_equal_and_project, and
// bit-blasted queries over check_relation relations.
func TestPlugins15SweepOutputs(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("test_vectors", "plugins15_sweep_*.smt2"))
	if err != nil || len(names) == 0 {
		t.Fatal("no vectors", err)
	}
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			script, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(name, ".smt2") + ".stdout")
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = run(bytes.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if out.String() != string(want) {
				t.Fatalf("got\n%s\nwant\n%s", out.String(), want)
			}
		})
	}
}
