package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2CheckSatUsingQFBVSLS(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Close()
	var output bytes.Buffer
	script := `(set-logic QF_BV)(declare-const p Bool)(assert p)(check-sat-using qfbv-sls)`
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\n" {
		t.Fatal(output.String())
	}
}
