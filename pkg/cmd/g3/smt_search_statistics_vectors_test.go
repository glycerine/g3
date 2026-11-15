package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// searchStatisticsVectors pins the search statistics `z3 -st` prints for
// vectors whose search used to diverge from the C++ (same answer, different
// conflicts/decisions/restarts).
var searchStatisticsVectors = []struct {
	name  string
	args  []string
	stats map[string]string
}{
	// AUDIT.md smtcore_06-F3: restart depth measured from m_base_lvl.
	{"restart_assumption_base_level.smt2", nil, map[string]string{"conflicts": "713", "decisions": "847", "restarts": "6"}},
}

var searchStatisticLine = regexp.MustCompile(`(?m)^\s*\(?:([a-z0-9-]+)\s+([0-9.]+)\)?\s*$`)

func searchStatistics(output string) map[string]string {
	stats := map[string]string{}
	for _, match := range searchStatisticLine.FindAllStringSubmatch(output, -1) {
		stats[match[1]] = match[2]
	}
	return stats
}

func TestSMTSearchStatisticsVectors(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	for _, vector := range searchStatisticsVectors {
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
