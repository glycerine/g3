package api

import (
	"strings"
	"testing"
)

func TestSMTContextRunOPBStatistics(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunOPBString("1 x1 >= 1;", &OPBRunOptions{DisplayStatistics: true})
	astTestMust(t, err)
	// Source statistics omit zero-valued entries. Memory rounded to hundredths
	// of a megabyte can be zero, particularly with debug allocator accounting.
	// opt_frontend display_statistics prints plain "name: value" lines and a
	// "time:" line (AUDIT.md FUZZ_J-F6).
	for _, fragment := range []string{"sat\n", "\nnum allocs:", "\ntime:                "} {
		if !strings.Contains(result.Output, fragment) {
			t.Fatalf("OPB statistics omitted %q: %s", fragment, result.Output)
		}
	}
}
