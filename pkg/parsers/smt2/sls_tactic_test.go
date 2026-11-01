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

func TestSMT2QFBVSLSBitVectorModel(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-logic QF_BV)
(declare-const x (_ BitVec 4))
(assert (= (bvand x #x3) #x2))
(check-sat-using (using-params qfbv-sls :random_seed 7))
(get-value ((= (bvand x #x3) #x2)))
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if want := "sat\n(((= (bvand x #x3) #x2) true))\n"; output.String() != want {
		t.Fatalf("SLS model output %q, want %q", output.String(), want)
	}
}
