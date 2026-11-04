package smt2

// Source differential: src/cmd_context/simplify_cmd.cpp.

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2SimplifyCommandEndToEnd(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	script := `(set-option :produce-proofs true)(set-logic QF_LIA)
(declare-const x Int)(declare-const y Int)
(simplify (+ 0 (* 1 x) 2 3))
(simplify (* (+ x y) (+ x y)) :som true)
(simplify (and true (= (+ 1 2) 3)))
(simplify (+ x 0) :print false)
(simplify (+ 1 2) :print-proofs true)`
	var output bytes.Buffer
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	want := "(+ 5 x)\n(+ (* x x) (* 2 x y) (* y y))\ntrue\n3\n(rewrite (= (+ 1 2) 3))\n"
	if output.String() != want {
		t.Fatal("simplify output", output.String(), "want", want)
	}
}

func TestSMT2SimplifyCommandStatistics(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	script := `(simplify (+ 1 2) :print false :print-statistics true)`
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	pattern := regexp.MustCompile(`^\(:time [0-9]+\.[0-9]{2} :num-steps 1 :memory [0-9]+\.[0-9]{2} :max-memory [0-9]+\.[0-9]{2} :cache-size: 0 :num-nodes-before 3 :num-shared 0 :num-nodes 1\)\n$`)
	if !pattern.MatchString(output.String()) {
		t.Fatal("simplify statistics", output.String())
	}
}

func TestSMT2SimplifyCommandRejectsUnknownParameterBeforeOutput(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	err = ParseCommands(context, strings.NewReader(`(simplify (+ 1 2) :not-a-parameter true)`), &output)
	if err == nil || !strings.Contains(err.Error(), "invalid keyword argument") || output.Len() != 0 {
		t.Fatal("simplify parameter refusal", err, output.String())
	}
}

func TestSMT2SimplifyCommandRecoversFromRewriterFailure(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	script := `(simplify (+ 1 2) :max_steps 0 :print_statistics true :timeout 4294967295)`
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	lines := strings.Split(output.String(), "\n")
	if len(lines) != 4 || lines[0] != `(error "simplifier failed: max. steps exceeded")` || lines[1] != "(+ 1 2)" || lines[3] != "" {
		t.Fatal("simplify recovery output", output.String())
	}
	pattern := regexp.MustCompile(`^\(:time [0-9]+\.[0-9]{2} :num-steps 1 :memory [0-9]+\.[0-9]{2} :max-memory [0-9]+\.[0-9]{2} :cache-size: 0 :num-nodes-before 3\)$`)
	if !pattern.MatchString(lines[2]) {
		t.Fatal("simplify recovery statistics", lines[2])
	}
}
