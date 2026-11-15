package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// AUDIT.md tactic_small_02-F2: mk_nra_tactic's quantifier-free arm is the
// three-attempt NLSAT or_else, so qe-lite is followed directly by the NLSAT
// preprocessing (simplifier, nnf) with no second simplify/propagate pass.
func TestSMTLibRunnerNRAQFArmIsNLSATPortfolio(t *testing.T) {
	script := `(set-logic NRA)
(declare-const x Real)
(declare-const y Real)
(declare-const z Real)
(assert (> (* x y) 2.0))
(assert (< (+ (* x x) (* y y z)) 7.0))
(assert (> (* z x) 1.0))
(check-sat)
`
	level, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	previous, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := diagnostic.SetProcessVerboseStream(previous); err != nil {
			t.Error(err)
		}
		if err := diagnostic.SetProcessVerbosityLevel(level); err != nil {
			t.Error(err)
		}
		if err := stream.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		t.Fatal(err)
	}
	if err = diagnostic.SetProcessVerbosityLevel(10); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if err = diagnostic.SetProcessVerbosityLevel(level); err != nil {
		t.Fatal(err)
	}
	verbose, err := stream.String()
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "sat\n" {
		t.Fatal("result", out.String())
	}
	lines := strings.Split(verbose, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "(qe-lite ") {
			continue
		}
		if i+2 >= len(lines) || !strings.HasPrefix(lines[i+1], "(simplifier ") || !strings.HasPrefix(lines[i+2], "(nnf ") {
			t.Fatalf("NRA arm after qe-lite:\n%s", verbose)
		}
		return
	}
	t.Fatalf("no qe-lite report:\n%s", verbose)
}
