package arith_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/tactic"
)

func nla2BVCanonical(text string) string {
	names := map[string]string{}
	text = regexp.MustCompile(`[^\s()]+![0-9]+`).ReplaceAllStringFunc(text, func(name string) string {
		if old, ok := names[name]; ok {
			return old
		}
		prefix := name[:strings.LastIndexByte(name, '!')]
		value := fmt.Sprintf("%s!aux%d", prefix, len(names))
		names[name] = value
		return value
	})
	return strings.Join(strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(text)), " ")
}

func TestNLA2BVSMTLIBSolvingAndModelsPrebuiltCPP(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	for _, tc := range []struct{ name, sort, assertions, formula, status string }{
		{"bounded-sat", "Int", `(assert (>= x 5))(assert (<= x 8))(assert (= (* x x) 49))`, `(= (* x x) 49)`, "sat"},
		{"bounded-unsat", "Int", `(assert (>= x 0))(assert (<= x 3))(assert (= (* x x) 2))`, "", "unsat"},
		{"unbounded-sat", "Int", `(assert (= (* x x) 4))`, `(= (* x x) 4)`, "sat"},
		{"unbounded-unknown", "Int", `(assert (= (* x x) 2))`, "", "unknown"},
		{"real-square", "Real", `(assert (= (* x x) 2.0))`, `(= (* x x) 2.0)`, "sat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := `(set-option :produce-models true)(set-logic ALL)(declare-const x ` + tc.sort + `)` + tc.assertions
			script := `(check-sat-using (then nla2bv simplify bit-blast sat))`
			if tc.status == "sat" {
				script += `(get-value (` + tc.formula + `))(get-model)`
			}
			c := cardContext(t, initial)
			var actual bytes.Buffer
			addBoundsMust(t, smt2.ParseCommands(c, strings.NewReader(script), &actual))
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(initial + script)
			native, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatal(err, string(native))
			}
			for name, text := range map[string]string{"Go": actual.String(), "C++": string(native)} {
				if strings.Contains(text, "error") || !strings.HasPrefix(strings.TrimSpace(text), tc.status+"\n") && strings.TrimSpace(text) != tc.status {
					t.Fatal(name, "solver status", text)
				}
				if tc.status == "sat" {
					if !strings.Contains(text, "true)") || !strings.Contains(text, "define-fun x") || strings.Contains(text, "!") {
						t.Fatal(name, "original model or auxiliary-symbol filtering", text)
					}
				}
			}
		})
	}
}

func TestNLA2BVExactGoalsPrebuiltCPP(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	for _, tc := range []struct {
		name, script string
		refuse       string
	}{
		{"bounded", `(declare-const x Int)(assert (>= x 5))(assert (<= x 8))(assert (<= (* x x) 49))`, ""},
		{"singleton", `(declare-const x Int)(assert (>= x 7))(assert (<= x 7))`, ""},
		{"unbounded", `(declare-const x Int)(assert (> x 0))`, ""},
		{"upper-only", `(declare-const x Int)(assert (<= x (- 5)))(assert (>= (* x x) 25))`, ""},
		{"centered", `(declare-const x Int)(declare-const y Int)(assert (= (* x x) (+ y 7)))`, ""},
		{"shared-variables", `(declare-const x Int)(declare-const y Int)(declare-const p Bool)(assert (>= x (- 2)))(assert (<= x 2))(assert (>= y 0))(assert (<= y 3))(assert (or p (= (* x y) (+ x x))))`, ""},
		{"real-square", `(declare-const x Real)(assert (= (* x x) 2.0))`, ""},
		{"real-comparison", `(declare-const x Real)(declare-const y Real)(assert (< (+ x 1.0) (* y y)))`, ""},
		{"real-ite", `(declare-const x Real)(declare-const p Bool)(assert (= (ite p x (- x)) 3.0))`, ""},
		{"integer-width-growth", `(declare-const x Int)(assert (= (* x x) 65537))`, ""},
		{"real-elimination-refusal", `(declare-const x Real)(assert (<= (+ x 0.2) 1.0))`, "nla2bv could not eliminate reals"},
		{"no-variables", `(assert true)`, "goal is not in the fragment supported by nla2bv"},
		{"uninterpreted-function", `(declare-const x Int)(declare-fun f (Int) Int)(assert (= (f x) 3))`, "goal is not in the fragment supported by nla2bv"},
		{"division", `(declare-const x Real)(assert (= (/ x 2.0) 3.0))`, "goal is not in the fragment supported by nla2bv"},
		{"quantifier", `(declare-const x Int)(assert (forall ((y Int)) (> (+ x y) y)))`, "goal is not in the fragment supported by nla2bv"},
		{"power-two-frontier", `(declare-const x Int)(declare-const y Int)(assert (or (= x 1) (= x 2) (= x 4)))(assert (> y 0))`, "goal is not in the fragment supported by nla2bv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := "(set-logic ALL)" + tc.script
			c := cardContext(t, script)
			m, err := c.Manager()
			addBoundsMust(t, err)
			g, err := tactic.NewGoal(m, false, true, false)
			addBoundsMust(t, err)
			t.Cleanup(func() { addBoundsMust(t, g.Close()) })
			addBoundsMust(t, c.AssertGoal(g))
			r, err := c.NewBuiltinTactic("nla2bv")
			addBoundsMust(t, err)
			addBoundsMust(t, r.IncRef())
			t.Cleanup(func() { addBoundsMust(t, r.DecRef()) })
			var result tactic.TacticResult
			t.Cleanup(func() { addBoundsMust(t, result.Close()) })
			err = r.Apply(g, &result)
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(script + "(apply nla2bv)")
			native, cppErr := cmd.CombinedOutput()
			if tc.refuse != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refuse) || !strings.Contains(string(native), tc.refuse) {
					t.Fatal("source refusal", err, cppErr, string(native))
				}
				return
			}
			addBoundsMust(t, err)
			if cppErr != nil {
				t.Fatal(cppErr, string(native))
			}
			if len(result.Goals) != 1 || result.Goals[0] != g || result.ModelConverter == nil || result.ProofConverter != nil {
				t.Fatal("source result metadata")
			}
			var actual bytes.Buffer
			actual.WriteString("(goals\n")
			addBoundsMust(t, g.DisplayWithAttributes(&actual))
			actual.WriteString(")\n")
			if nla2BVCanonical(actual.String()) != nla2BVCanonical(string(native)) {
				t.Fatalf("tactic goals differ\nGo: %s\nC++: %s", actual.String(), native)
			}
		})
	}
}

func TestNLA2BVPrecisionMatchesCPPReference(t *testing.T) {
	reference := os.Getenv("G3_CPP_REFERENCE")
	if reference == "" {
		t.Skip("set G3_CPP_REFERENCE to prebuilt z3")
	}
	for _, tc := range []struct {
		name, logic, declarations, assertions string
		want                                  string
	}{
		{"bounded-integer", "NIA", `(declare-const x Int)`, `(assert (>= x 3))(assert (<= x 3))(assert (= (* x x) 9))`, "precise"},
		{"equality-integer", "NIA", `(declare-const x Int)`, `(assert (= x 3))`, "precise"},
		{"unbounded-integer", "NIA", `(declare-const x Int)`, `(assert (= (* x x) 9))`, "under"},
		{"real", "NRA", `(declare-const x Real)`, `(assert (= x 1.0))`, "under"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initial := `(set-logic ` + tc.logic + `)` + tc.declarations + tc.assertions
			c := cardContext(t, initial)
			m, err := c.Manager()
			addBoundsMust(t, err)
			g, err := tactic.NewGoal(m, false, true, false)
			addBoundsMust(t, err)
			t.Cleanup(func() { addBoundsMust(t, g.Close()) })
			addBoundsMust(t, c.AssertGoal(g))
			selected, err := c.NewBuiltinTactic("nla2bv")
			addBoundsMust(t, err)
			addBoundsMust(t, selected.IncRef())
			t.Cleanup(func() { addBoundsMust(t, selected.DecRef()) })
			var result tactic.TacticResult
			t.Cleanup(func() { addBoundsMust(t, result.Close()) })
			addBoundsMust(t, selected.Apply(g, &result))
			precision, err := g.Precision()
			addBoundsMust(t, err)
			want := tactic.GoalPrecise
			if tc.want == "under" {
				want = tactic.GoalUnder
			}
			if precision != want {
				t.Fatalf("Go goal precision = %v, want %s", precision, tc.want)
			}
			cmd := exec.Command(reference, "-in")
			cmd.Stdin = strings.NewReader(initial + `(apply nla2bv)`)
			native, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(native), ":precision "+tc.want) {
				t.Fatalf("C++ precision = %s, error = %v, want %s", native, err, tc.want)
			}
		})
	}
}
