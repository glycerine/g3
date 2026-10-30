package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTContextRunOPBReturnsCopiedHardResult(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	result, err := f.c.RunOPB(strings.NewReader("1 x1 >= 1;"), nil)
	astTestMust(t, err)
	if result.Status != lbool.True || result.Summary != (OPBLoadSummary{Variables: 1, Constraints: 1}) {
		t.Fatal("OPB hard result", result.Status, result.Summary)
	}
	if len(result.Document.Constraints) != 1 || len(result.Document.Constraints[0].Terms[0].Literals) != 1 || result.Document.Constraints[0].Terms[0].Literals[0].Variable != 1 {
		t.Fatal("OPB copied document", result.Document)
	}
	if result.Output != "sat\n" || result.Elapsed <= 0 {
		t.Fatal("OPB output/timing", result.Output, result.Elapsed)
	}
}
