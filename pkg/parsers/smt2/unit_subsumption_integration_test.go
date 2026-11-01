package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2UnitSubsumptionPreservesUnsatCore(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :produce-unsat-cores true)
(set-logic QF_UF)
(declare-const p Bool)
(declare-const q Bool)
(assert (! (or p q) :named weak))
(assert (! p :named positive))
(assert (! (not p) :named negative))
(check-sat-using (then unit-subsume-simplify smt))
(get-unsat-core)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.HasPrefix(got, "unsat\n") || !strings.Contains(got, "positive") || !strings.Contains(got, "negative") {
		t.Fatalf("unit subsumption unsat core: %q", got)
	}
}

func TestSMT2UnitSubsumptionProducesProof(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :produce-proofs true)
(set-logic QF_UF)
(declare-const p Bool)
(declare-const q Bool)
(assert (or p q))
(assert p)
(assert (not p))
(check-sat-using (then unit-subsume-simplify smt))
(get-proof)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.HasPrefix(got, "unsat\n") || !strings.Contains(got, "asserted") {
		t.Fatalf("unit subsumption proof: %q", got)
	}
}
