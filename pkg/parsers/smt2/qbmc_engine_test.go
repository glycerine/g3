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

func TestSMT2QBMCProofConvertsSlicedPredicate(t *testing.T) {
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
(declare-rel P (Int Int))
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (P x y))
(rule (=> (P x y) (Q y)))
(rule (=> (Q 0) Goal))
(query Goal :print-certificate true)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "(P ") || strings.Contains(text, "!slice!") {
		t.Fatal("expected certificate using original predicates", text)
	}
}

func TestSMT2QBMCNonlinearSlicedProofTrace(t *testing.T) {
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
(declare-rel Q (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (P x))
(rule (Q y))
(rule (=> (and (P x) (Q y)) Goal))
(query Goal :print-certificate true)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "(P ") || !strings.Contains(text, "(Q ") || strings.Contains(text, "!slice!") {
		t.Fatal("nonlinear sliced certificate", text)
	}
}

func TestSMT2QBMCProofConvertsMergedTails(t *testing.T) {
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
(declare-rel Q (Int))
(declare-rel R (Int))
(declare-rel Goal ())
(declare-var x Int)
(declare-var y Int)
(rule (P 0))
(rule (Q 0))
(rule (R y))
(rule (=> (and (P x) (Q x) (Q 0) (R y)) Goal))
(query Goal :print-certificate true)
`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	text := output.String()
	if !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, "hyper-res") || !strings.Contains(text, "(P ") || !strings.Contains(text, "(Q ") || strings.Contains(text, "!slice!") {
		t.Fatal("merged-tail certificate", text)
	}
}
