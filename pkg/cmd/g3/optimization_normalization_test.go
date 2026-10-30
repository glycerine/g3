package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Source: src/opt/opt_context.cpp::is_maxsat/from_fmls and objective value adjustment.
func TestOptimizationNormalizedIndicatorObjectives(t *testing.T) {
	for _, priority := range []string{"lex", "box", "pareto"} {
		for _, tc := range []struct{ name, direction, expression, value string }{
			{"min", "minimize", "(+ 7 (ite p 1 0) (ite q 2 0))", "8"},
			{"max", "maximize", "(+ 7 (ite p 1 0) (ite q 2 0))", "10"},
			{"negative-min", "minimize", "(+ 7 (ite p (- 3) 0) (ite q 2 0))", "4"},
		} {
			t.Run(priority+"/"+tc.name, func(t *testing.T) {
				script := fmt.Sprintf(`(set-option :produce-models true)
(set-option :opt.priority %s)
(declare-const p Bool)(declare-const q Bool)
(define-fun cost () Int %s)
(assert (or p q))(%s cost)
(check-sat)(get-objectives)(get-value (cost))`, priority, tc.expression, tc.direction)
				var out bytes.Buffer
				if err := run(strings.NewReader(script), &out); err != nil {
					t.Fatal(err, out.String())
				}
				if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
					command := exec.Command(reference, "-in")
					command.Stdin = strings.NewReader(script)
					native, err := command.CombinedOutput()
					if err != nil || !strings.Contains(string(native), "((cost "+tc.value+"))") {
						t.Fatalf("C++ reference: %v\n%s", err, native)
					}
				}
				if !strings.HasPrefix(out.String(), "sat\n") || !strings.Contains(out.String(), " "+tc.value+")\n") || !strings.Contains(out.String(), "((cost "+tc.value+"))") {
					t.Fatalf("normalized objective/model result: %s", out.String())
				}
			})
		}
	}
}

func TestOptimizationNormalizedBoxContinuation(t *testing.T) {
	script := `(set-option :produce-models true)(set-option :opt.priority box)
(declare-const p Bool)(declare-const q Bool)
(define-fun cost () Int (+ 7 (ite p 1 0) (ite q 2 0)))
(assert (or p q))(minimize cost)(maximize cost)
(check-sat)(get-value (cost))(check-sat)(get-value (cost))(check-sat)`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if out.String() != "sat\n((cost 8))\nsat\n((cost 10))\nunsat\n" {
		t.Fatal(out.String())
	}
}
