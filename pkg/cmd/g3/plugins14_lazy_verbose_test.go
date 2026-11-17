//go:build g3trace

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var plugins14VerboseTime = regexp.MustCompile(`(?m)([0-9][0-9.e+-]*)s$`)

// TestPlugins14LazyVerbose: lazy_sparse transitive closures (join and
// select_equal_and_project) at -v:12 against the fixed fork's stderr
// (plugins14_<name>.v12.txt, without the "(transform ...)" lines, times
// as 0s): the lazy table forces' verbose actions (join, join_project,
// rename, union, clone, filter_equal, select_equal_project, ...) and the
// sparse tables' Recycle lines in the order the lazy select deallocates
// them (instr_select_equal_and_project sets the register before
// fast_empty forces the result).
func TestPlugins14LazyVerbose(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lz_join_16", "lz_sel_32"} {
		want, err := os.ReadFile(filepath.Join(wd, "test_vectors", "plugins14_"+name+".v12.txt"))
		if err != nil {
			t.Fatal(err)
		}
		log := runVerboseProgressChild(t, []string{"-v:12", filepath.Join(wd, "test_vectors", "plugins14_"+name+".smt2")})
		var got []string
		for _, line := range strings.Split(log, "\n") {
			if !strings.HasPrefix(line, "(transform") {
				got = append(got, line)
			}
		}
		if g := plugins14VerboseTime.ReplaceAllString(strings.Join(got, "\n"), "0s"); g != string(want) {
			t.Fatalf("%s: -v:12 lines\n%s\nwant\n%s", name, g, want)
		}
	}
}
