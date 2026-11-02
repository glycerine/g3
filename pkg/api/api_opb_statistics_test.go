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
	for _, fragment := range []string{"sat\n", ":total-time", ":num-allocs"} {
		if !strings.Contains(result.Output, fragment) {
			t.Fatalf("OPB statistics omitted %q: %s", fragment, result.Output)
		}
	}
}
