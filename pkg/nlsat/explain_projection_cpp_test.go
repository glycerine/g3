package nlsat

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestProjectionCoefficientsAndPSCPrebuiltCPPOracle(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	var script strings.Builder
	script.WriteString("(set-logic QF_NRA)\n(declare-const x0 Real)\n(declare-const x1 Real)\n")
	queries := 0
	query := func(formula string) {
		fmt.Fprintf(&script, "(push)(assert %s)(check-sat-using qfnra-nlsat)(pop)\n", formula)
		queries++
	}
	for _, factor := range []bool{false, true} {
		for root := int32(-3); root <= 3; root++ {
			e, _, n, todo := projectionCoefficientFixture(t, factor)
			evaluatorSetFraction(t, e, 0, int64(root+5), 1)
			terms := []evaluatorTerm{{1, 2, 3}, {1, 0, 1}, {1, 0, 0}}
			if 2-root != 0 {
				terms = append(terms, evaluatorTerm{2 - root, 1, 3})
			}
			if root != 0 {
				terms = append(terms, evaluatorTerm{-2 * root, 0, 3})
			}
			p := evaluatorPolynomial(t, e, terms...)
			ps := projectionOutput(t, e)
			assignmentMust(t, ps.Push(p))
			assignmentMust(t, n.collectLeadingCoefficients(ps, 1))
			var zeros []string
			for _, q := range projectionValues(t, todo.set) {
				zeros = append(zeros, "(= "+explanationPolynomialSMT(t, e, q)+" 0.0)")
			}
			if len(zeros) == 0 {
				t.Fatal("projection LC absent")
			}
			actual := "(or " + strings.Join(zeros, " ") + ")"
			lc := "(* (- x0 " + intervalSMTInteger(root) + ") (+ x0 2.0))"
			query("(not (= " + actual + " (= " + lc + " 0.0)))")
			query("(and (= x0 " + intervalSMTInteger(root+5) + ") " + actual + ")")
			input := explanationPolynomialSMT(t, e, p)
			independent := "(+ (* " + lc + " x1 x1 x1) x1 1.0)"
			query("(not (= " + input + " " + independent + "))")
		}
	}
	for root := int32(-3); root <= 3; root++ {
		for _, vanishes := range []bool{false, true} {
			e, _, n, todo := projectionCoefficientFixture(t, true)
			value := root + 1
			if vanishes {
				value = root
			}
			evaluatorSetFraction(t, e, 0, int64(value), 1)
			p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 1})
			terms := []evaluatorTerm{{1, 0, 1}, {1, 1, 0}}
			if root != 0 {
				terms = append(terms, evaluatorTerm{-root, 0, 0})
			}
			q := evaluatorPolynomial(t, e, terms...)
			assignmentMust(t, n.collectProjectionPSC(vanishingRef(t, e, p), vanishingRef(t, e, q), 1))
			inputP, inputQ := explanationPolynomialSMT(t, e, p), explanationPolynomialSMT(t, e, q)
			values := projectionValues(t, todo.set)
			if len(values) != 1 {
				t.Fatal("missing linear PSC boundary", values)
			}
			boundary := explanationPolynomialSMT(t, e, values[0])
			query("(not (= (= " + boundary + " 0.0) (= x0 " + intervalSMTInteger(root) + ")))")
			query("(and (= " + inputP + " 0.0) (= " + inputQ + " 0.0) (not (= " + boundary + " 0.0)))")
		}
	}
	e, _, n, todo := projectionCoefficientFixture(t, false)
	evaluatorSetFraction(t, e, 0, 1, 1)
	p := evaluatorPolynomial(t, e, evaluatorTerm{1, 0, 2}, evaluatorTerm{1, 1, 0})
	q := evaluatorPolynomial(t, e, evaluatorTerm{1, 1, 2}, evaluatorTerm{1, 1, 1}, evaluatorTerm{1, 0, 0})
	assignmentMust(t, n.collectProjectionPSC(vanishingRef(t, e, p), vanishingRef(t, e, q), 1))
	values := projectionValues(t, todo.set)
	if len(values) != 1 {
		t.Fatal("quartic PSC frontier")
	}
	selected := explanationPolynomialSMT(t, e, values[0])
	quartic := "(+ (* x0 x0 x0 x0) (* x0 x0 x0) (- (* 2.0 x0 x0)) 1.0)"
	query("(not (= (= " + selected + " 0.0) (= " + quartic + " 0.0)))")
	query("(and (= " + explanationPolynomialSMT(t, e, p) + " 0.0) (= " + explanationPolynomialSMT(t, e, q) + " 0.0) (not (= " + selected + " 0.0)))")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, reference, "-in", "-smt2")
	cmd.Stdin = strings.NewReader(script.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("C++ projection oracle %v: %s", err, out)
	}
	answers := strings.Fields(string(out))
	if len(answers) != queries {
		t.Fatalf("C++ answers %d want %d: %s", len(answers), queries, out)
	}
	for i, a := range answers {
		if a != "unsat" {
			t.Fatal("projection counterexample", i, a)
		}
	}
}
