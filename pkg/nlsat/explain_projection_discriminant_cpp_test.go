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

func TestProjectionDiscriminantsPrebuiltCPPOracle(t *testing.T) {
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
	for _, degree := range []uint32{2, 3} {
		for root := int32(-2); root <= 2; root++ {
			for _, vanishes := range []bool{false, true} {
				for _, factor := range []bool{false, true} {
					e, _, n, todo := projectionCoefficientFixture(t, factor)
					value := root + 2
					if vanishes {
						value = root
					}
					evaluatorSetFraction(t, e, 0, int64(value), 1)
					terms := []evaluatorTerm{{1, 0, degree}, {-1, 1, 0}}
					if root != 0 {
						terms = append(terms, evaluatorTerm{root, 0, 0})
					}
					p := evaluatorPolynomial(t, e, terms...)
					ps := projectionOutput(t, e)
					assignmentMust(t, ps.Push(p))
					assignmentMust(t, n.collectProjectionDiscriminants(ps, 1))
					power := strings.TrimSpace(strings.Repeat("x1 ", int(degree)))
					dpPower := strings.TrimSpace(strings.Repeat("x1 ", int(degree-1)))
					independent := "(+ (* " + power + ") (- x0) " + intervalSMTInteger(root) + ")"
					actual := explanationPolynomialSMT(t, e, p)
					query("(not (= " + actual + " " + independent + "))")
					var zeros []string
					for _, s := range projectionValues(t, todo.set) {
						zeros = append(zeros, "(= "+explanationPolynomialSMT(t, e, s)+" 0.0)")
					}
					if len(zeros) == 0 {
						t.Fatal("discriminant polynomials absent")
					}
					zero := "(or " + strings.Join(zeros, " ") + ")"
					derivative := fmt.Sprintf("(* %d %s)", degree, dpPower)
					query("(not (= " + zero + " (= x0 " + intervalSMTInteger(root) + ")))")
					query("(and (= " + actual + " 0.0) (= " + derivative + " 0.0) (not " + zero + "))")
				}
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, reference, "-in", "-smt2")
	cmd.Stdin = strings.NewReader(script.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("C++ discriminant oracle %v: %s", err, out)
	}
	answers := strings.Fields(string(out))
	if len(answers) != queries {
		t.Fatalf("C++ answers %d want %d: %s", len(answers), queries, out)
	}
	for i, a := range answers {
		if a != "unsat" {
			t.Fatal("discriminant disagreement", i, a)
		}
	}
}
