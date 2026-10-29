package cmd_context

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestLoadOPBMinObjectiveUsesWeightedIndicatorSum(t *testing.T) {
	document, err := opt.ParseOPB(strings.NewReader("min: 1 x1 +2 x2; 1 x1 +1 x2 >= 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := opbTestContext(t)
	summary, err := c.LoadOPB(document)
	if err != nil {
		t.Fatal(err)
	}
	if summary != (OPBLoadSummary{Variables: 2, Constraints: 1, Objectives: 1}) {
		t.Fatal(summary)
	}
	status, err := c.CheckSat(nil)
	if err != nil || status != lbool.True {
		t.Fatal(status, err)
	}
	var output bytes.Buffer
	if err = c.WriteObjectives(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), " 1)") {
		t.Fatalf("objective = %q", output.String())
	}
}

func TestLoadOPBMaxSyntaxRetainsSourceSubtraction(t *testing.T) {
	document, err := opt.ParseOPB(strings.NewReader("max: 10 x1 +3 x2; 1 x1 = 0; 1 x2 = 1;"))
	if err != nil {
		t.Fatal(err)
	}
	c := opbTestContext(t)
	if _, err = c.LoadOPB(document); err != nil {
		t.Fatal(err)
	}
	if _, err = c.CheckSat(nil); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = c.WriteObjectives(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), " (- 3))") && !strings.Contains(output.String(), " -3)") {
		t.Fatalf("source max subtraction objective = %q", output.String())
	}
}
