//go:build g3trace

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var plugins15VerboseTime = regexp.MustCompile(`(?m)[0-9][0-9.e+-]*s(\)?)$`)

// TestPlugins15SweepVerbose: the plugins15 sweep programs at -v:12 against
// the fixed fork's stderr (plugins15_sweep_<name>.v12.txt, times as 0s):
// no Recycle or check_relation lines for relations the source never
// allocates (placeholders, bit-blast results copied back), the unbound
// compressor registered when check_relation replaces a doc default
// relation, doc relations over checked default tables, and udoc's size
// estimates (sizeof(udoc_relation), sizeof(doc)).
func TestPlugins15SweepVerbose(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("test_vectors", "plugins15_sweep_*.smt2"))
	if err != nil || len(names) == 0 {
		t.Fatal("no vectors", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			want, err := os.ReadFile(strings.TrimSuffix(name, ".smt2") + ".v12.txt")
			if err != nil {
				t.Fatal(err)
			}
			got := runVerboseProgressChild(t, []string{"-v:12", filepath.Join(wd, name)})
			if got = plugins15VerboseTime.ReplaceAllString(got, "0s$1"); got != string(want) {
				t.Fatalf("%s -v:12\n%s\nwant\n%s", name, got, want)
			}
		})
	}
}
