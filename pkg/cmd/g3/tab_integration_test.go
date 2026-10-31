package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerTabulationQueries(t *testing.T) {
	script := `(set-option :fixedpoint.engine tab)
(declare-rel p ())
(declare-rel q ())
(declare-rel missing ())
(rule p)
(rule (=> p q))
(query q)
(query missing)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "sat\nunsat" {
		t.Fatalf("tabulation result: %s", got)
	}
}

func TestSMTLibRunnerTabulationArithmeticRules(t *testing.T) {
	script := `(set-option :fixedpoint.engine tab)
(declare-var x Int)
(declare-rel p (Int))
(declare-rel goal ())
(rule (p 0))
(rule (=> (and (p x) (< x 2)) (p (+ x 1))))
(rule (=> (p 2) goal))
(query goal)
(query p)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "sat\nsat" {
		t.Fatalf("tabulation arithmetic result: %s", got)
	}
}

func TestSMTLibRunnerTabulationRecursiveCycle(t *testing.T) {
	script := `(set-option :fixedpoint.engine tab)
(declare-rel loop ())
(rule (=> loop loop))
(query loop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "unsat" {
		t.Fatalf("tabulation recursive result: %s", got)
	}
}

func TestSMTLibRunnerTabulationSelectionOptions(t *testing.T) {
	for _, strategy := range []string{"weight", "basic-weight", "first", "var-use"} {
		t.Run(strategy, func(t *testing.T) {
			script := "(set-option :fixedpoint.engine tab)\n(set-option :fixedpoint.tab.selection " + strategy + ")\n" +
				"(declare-rel p ())\n(declare-rel q ())\n(declare-rel goal ())\n(rule p)\n(rule q)\n(rule (=> (and p q) goal))\n(query goal)\n"
			var out bytes.Buffer
			if err := run(strings.NewReader(script), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if got := strings.TrimSpace(out.String()); got != "sat" {
				t.Fatal("strategy result", got)
			}
		})
	}
}
