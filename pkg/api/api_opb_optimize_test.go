package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextRunOPBStringOptimizes(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunOPBString("min: 1 x1 +2 x2; 1 x1 +1 x2 >= 1;", nil)
	astTestMust(t, err)
	if result.Status != lbool.True || result.Summary != (OPBLoadSummary{Variables: 2, Constraints: 1, Objectives: 1}) {
		t.Fatal("OPB optimize result", result.Status, result.Summary)
	}
	if !strings.HasPrefix(result.Output, "sat\n(objectives\n") || !strings.Contains(result.Output, " 1)") {
		t.Fatal("OPB objective output", result.Output)
	}
}
