package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerInjectivityTactic(t *testing.T) {
	script := `(declare-sort U 0)
(declare-fun f (U) U)
(declare-fun g (U) U)
(declare-const a U)
(declare-const b U)
(assert (forall ((x U)) (= (g (f x)) x)))
(assert (= (f a) (f b)))
(apply injectivity)`
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if !strings.Contains(output.String(), "(= a b)") {
		t.Fatal("injectivity rewrite missing:", output.String())
	}
}
