package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins16SweepOutputs: larger plugins16 sweep programs whose answers
// differed from the fixed fork's (plugins16_out_<name>.stdout): udoc's
// filter_interpreted and filter_interpreted_and_project functors applied
// more than once, and the unbound compressor's compressed tails.
func TestPlugins16SweepOutputs(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("test_vectors", "plugins16_out_*.smt2"))
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
