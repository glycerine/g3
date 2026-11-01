package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2QBMCEngineSearchAndCertificate(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine qbmc)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel P (Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-rel Empty ())
(declare-var x Int)
(rule (P 3))
(rule (=> (P x) (Q x)))
(rule (=> (Q 3) Goal))
(query Goal :print-certificate true)
(query Empty)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.HasSuffix(text, "unsat\n") {
		t.Fatalf("QBMC certificate and empty query: %s", text)
	}
}

func TestSMT2QBMCNonlinearFallsBackToNonlinearCompiler(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine qbmc)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel A ())
(declare-rel B ())
(declare-rel Goal ())
(rule A)
(rule B)
(rule (=> (and A B) Goal))
(query Goal)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "sat\n" {
		t.Fatalf("nonlinear QBMC result: %s", output.String())
	}
}

func TestSMT2QBMCRequestedProofTrace(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine qbmc)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel A ())
(declare-rel Goal ())
(rule A)
(rule (=> A Goal))
(query Goal :print-certificate true)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "asserted") {
		t.Fatalf("QBMC proof trace: %s", text)
	}
}

func TestSMT2QBMCProofWithUnconstrainedRuleVariable(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	script := `(set-option :fixedpoint.engine qbmc)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(set-option :fixedpoint.xform.subsumption_checker false)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (P x))
(rule (=> (P x) Goal))
(query Goal :print-certificate true)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "asserted") || strings.Contains(text, "!slice!") {
		t.Fatalf("QBMC proof over original predicates: %s", text)
	}
}
