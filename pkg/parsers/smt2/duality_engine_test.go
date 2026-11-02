package smt2_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func TestSMT2DualityRecursiveIntegerQueries(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	script := `(set-option :fixedpoint.engine duality)
(declare-rel Reach (Int))
(declare-rel Missing ())
(declare-rel GoalTwo ())
(declare-rel GoalOne ())
(declare-var x Int)
(declare-var y Int)
(rule (Reach 0))
(rule (=> (and (Reach x) (= y (+ x 1))) (Reach y)))
(rule (=> (Reach 2) GoalTwo))
(rule (=> (Reach 1) GoalOne))
(query GoalTwo)
(query Missing)
(query GoalOne)
`
	var output bytes.Buffer
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "sat\nunsat\nsat\n" {
		t.Fatalf("Duality query results: %q", output.String())
	}
}

func TestSMT2DualitySafetyCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(declare-rel P ())
(declare-rel Q ())
(rule P)
(query Q :print-answer false :print-certificate true)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "unsat\n(fixedpoint \n") || !strings.Contains(output.String(), "define-fun Q") {
		t.Fatalf("safety certificate: %q", output.String())
	}
}
