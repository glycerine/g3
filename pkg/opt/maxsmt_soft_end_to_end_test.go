package opt_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestMaxSMTDuplicateAndNegativeWeightsEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Close()
	script := `(declare-const p Bool)
(assert (not p))
(assert-soft p :weight 2)
(assert-soft p :weight 3)
(assert-soft (not p) :weight -7)
(check-sat)
(get-objectives)
`
	var output bytes.Buffer
	if err = smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "sat\n(objectives\n ( 5)\n)\n" {
		t.Fatalf("output %q", output.String())
	}
}
