package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// lemmaGCStatisticsVectors pins the search statistics `z3 -st` prints for
// vectors whose lemma garbage collection used to diverge from the C++.
var lemmaGCStatisticsVectors = []struct {
	name  string
	args  []string
	stats map[string]string
}{
	// AUDIT.md smtcore_02-F10 (follow-up): setup_QF_UFIDL's m_lemma_gc_half
	// must reach del_inactive_lemmas, whose first collection then deletes
	// half of the old lemmas.
	{"lemma_gc_half_dense_setup.smt2", nil, map[string]string{
		"conflicts": "5003", "decisions": "6742", "del-clause": "2450", "mk-clause": "5401",
		"propagations": "85344", "minimized-lits": "16814", "restarts": "18", "dd-assertions": "76372"}},
}

func TestLemmaGCStatisticsVectors(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, vector := range lemmaGCStatisticsVectors {
		t.Run(vector.name, func(t *testing.T) {
			input := filepath.Join("test_vectors", vector.name)
			args := append(append([]string{"-st"}, vector.args...), input)
			var out bytes.Buffer
			if err := runArguments(args, strings.NewReader(""), &out); err != nil {
				t.Fatal(err, out.String())
			}
			got := searchStatistics(out.String())
			for key, want := range vector.stats {
				if got[key] != want {
					t.Fatalf(":%s = %q, want %q\n%s", key, got[key], want, out.String())
				}
			}
			if reference == "" {
				return
			}
			native, err := exec.Command(reference, args...).CombinedOutput()
			if err != nil {
				t.Fatal("C++ reference", err, string(native))
			}
			cpp := searchStatistics(string(native))
			for key, want := range vector.stats {
				if cpp[key] != want {
					t.Fatalf("C++ :%s = %q, want %q\n%s", key, cpp[key], want, native)
				}
			}
		})
	}
}
