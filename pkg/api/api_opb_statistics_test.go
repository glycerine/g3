package api

import (
	"strings"
	"testing"
)

func TestSMTContextRunOPBStatistics(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunOPBString("1 x1 >= 1;", &OPBRunOptions{DisplayStatistics: true})
	astTestMust(t, err)
	for _, fragment := range []string{"sat\n", ":total-time", ":memory"} {
		if !strings.Contains(result.Output, fragment) {
			t.Fatalf("OPB statistics omitted %q: %s", fragment, result.Output)
		}
	}
}
