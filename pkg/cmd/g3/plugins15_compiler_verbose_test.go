//go:build g3trace

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins15CompilerVerbose: lazy_sparse transitive closures at -v:12
// against the fixed fork's stderr (plugins15_<name>.v12.txt, without the
// "(transform ...)" lines, times as 0s). lz_sel_64: mk_similarity_compressor's
// add_fact resets the saturation marks, so only the last merged class's
// auxiliary predicate stays saturated and the others' strata are compiled
// (mark_saturated b!sc_1!0). lz_sel_16: compile_dependent_rules' register
// numbering and order (detect_chains' preds_vector, get_fresh_registers over
// the stratum's copy, the delta maps' table order for the loop controls and
// make_inloop_delta_transition's moves).
func TestPlugins15CompilerVerbose(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lz_sel_64", "lz_sel_16"} {
		want, err := os.ReadFile(filepath.Join(wd, "test_vectors", "plugins15_"+name+".v12.txt"))
		if err != nil {
			t.Fatal(err)
		}
		log := runVerboseProgressChild(t, []string{"-v:12", filepath.Join(wd, "test_vectors", "plugins15_"+name+".smt2")})
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
