package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestFixedpointQPDRQuantifiedRules(t *testing.T) {
	script := `(set-option :fixedpoint.engine qpdr)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel P ())
(declare-rel Q ())
(rule (=> (exists ((x Int)) (= x 1)) P))
(rule (=> (exists ((x Int)) (and (= x 1) (= x 2))) Q))
(query P)
(query Q)
`
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.Join(strings.Fields(output.String()), " "); got != "sat unsat" {
		t.Fatalf("quantified PDR results: %s", got)
	}
}

func TestFixedpointQPDRQuantifiedBodyUsesRuleVariable(t *testing.T) {
	script := `(set-option :fixedpoint.engine qpdr)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel P (Int))
(declare-rel Goal ())
(declare-var x Int)
(rule (=> (exists ((y Int)) (and (> y x) (< y (+ x 2)))) (P x)))
(rule (=> (P 3) Goal))
(query Goal)
`
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.TrimSpace(output.String()); got != "sat" {
		t.Fatalf("quantified PDR result: %s", got)
	}
}

func TestFixedpointAutoQPDRQuantifiedRule(t *testing.T) {
	script := `(set-option :fixedpoint.engine auto-config)
(set-option :fixedpoint.xform.slice false)
(set-option :fixedpoint.xform.inline_eager false)
(set-option :fixedpoint.xform.inline_linear false)
(declare-rel P ())
(rule (=> (exists ((x Int)) (= x 1)) P))
(query P)
`
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.TrimSpace(output.String()); got != "sat" {
		t.Fatalf("automatic quantified PDR result: %s", got)
	}
}

func TestFixedpointAutoEmptyInfiniteRelation(t *testing.T) {
	script := `(set-option :fixedpoint.engine auto-config)
(declare-rel P (Int))
(query P)
`
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.TrimSpace(output.String()); got != "unsat" {
		t.Fatalf("empty infinite relation result: %s", got)
	}
}
