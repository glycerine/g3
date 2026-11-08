package smtlogics_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func qfliaContext(t *testing.T, script string) *cmd_context.Context {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	qfaufliaMust(t, err)
	var budget *arena.Budget
	t.Cleanup(func() {
		qfaufliaMust(t, c.Close())
		if budget != nil && budget.Used() != 0 {
			t.Errorf("QF_LIA context retained %d bytes", budget.Used())
		}
	})
	qfaufliaMust(t, smt2.ParseCommands(c, strings.NewReader(script), io.Discard))
	m, err := c.Manager()
	qfaufliaMust(t, err)
	budget = m.Budget()
	return c
}

func TestQFLIATacticModelSurvivesStrategyClose(t *testing.T) {
	c := qfliaContext(t, `(set-logic QF_LIA)(set-option :produce-models true)
(declare-const x Int)(assert (>= x 5))(assert (<= x 7))
(assert (not (= x 5)))(assert (not (= x 6)))`)
	m, err := c.Manager()
	qfaufliaMust(t, err)
	x, err := c.MakeApp("x", nil, nil, ast.ASTHandle{})
	qfaufliaMust(t, err)
	r, err := c.NewBuiltinTactic("qflia")
	qfaufliaMust(t, err)
	qfaufliaMust(t, r.IncRef())
	closed := false
	t.Cleanup(func() {
		if !closed {
			qfaufliaMust(t, r.DecRef())
		}
	})
	g, err := tactic.NewGoal(m, false, true, false)
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, g.Close()) })
	qfaufliaMust(t, c.AssertGoal(g))
	var result tactic.TacticResult
	t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
	qfaufliaMust(t, r.Apply(g, &result))
	if !result.DecidedSat() || result.ModelConverter == nil {
		t.Fatal("strategy did not produce a reconstructable model")
	}
	qfaufliaMust(t, r.Cleanup())
	qfaufliaMust(t, r.DecRef())
	closed = true
	var md tactic.ModelRef
	t.Cleanup(func() { qfaufliaMust(t, md.Close()) })
	qfaufliaMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &md))
	model, err := md.Get()
	qfaufliaMust(t, err)
	d, err := m.GetDecl(x)
	qfaufliaMust(t, err)
	value, err := model.ConstantInterpretation(d)
	qfaufliaMust(t, err)
	text, integer, matched, err := m.ArithmeticNumeralText(value)
	qfaufliaMust(t, err)
	if !integer || !matched || text != "7" {
		t.Fatal("shifted integer reconstruction after strategy destruction", text)
	}
}

func TestQFLIATacticTranslatedProofAndCoreRoutes(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			option := "produce-unsat-cores"
			if proofs {
				option = "produce-proofs"
			}
			c := qfliaContext(t, "(set-logic QF_LIA)(set-option :"+option+" true)(declare-const x Int)(assert (! (>= x 3) :named lo))(assert (! (<= x 2) :named hi))")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			r, err := c.NewBuiltinTactic("qflia")
			qfaufliaMust(t, err)
			qfaufliaMust(t, r.IncRef())
			mode := ast.ASTProofDisabled
			if proofs {
				mode = ast.ASTProofEnabled
			}
			target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget(), ProofMode: mode})
			qfaufliaMust(t, err)
			// The source copy-constructs the target (ast_manager(m, ...)), which
			// keeps the source family ids.
			qfaufliaMust(t, target.CopyFamiliesPlugins(m))
			t.Cleanup(func() { qfaufliaMust(t, target.Close()) })
			translated, err := r.Translate(target)
			qfaufliaMust(t, err)
			qfaufliaMust(t, translated.IncRef())
			qfaufliaMust(t, r.DecRef())
			t.Cleanup(func() { qfaufliaMust(t, translated.DecRef()) })
			g, err := tactic.NewGoal(m, proofs, false, !proofs)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, g.Close()) })
			qfaufliaMust(t, c.AssertGoal(g))
			translation, err := ast.NewASTTranslation(m, target, true)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, translation.Close()) })
			copy, err := g.Translate(translation)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, copy.Close()) })
			var model tactic.ModelRef
			t.Cleanup(func() { qfaufliaMust(t, model.Close()) })
			var labels tactic.ModelLabels
			t.Cleanup(labels.Close)
			pr, err := target.NewRef(ast.ASTHandle{})
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, target.DelRef(&pr)) })
			var core tactic.DependencyRef
			t.Cleanup(func() { qfaufliaMust(t, core.Close()) })
			status, reason, err := tactic.CheckSat(translated, copy, &model, &labels, pr, &core)
			qfaufliaMust(t, err)
			if status != lbool.False {
				t.Fatal(status, reason)
			}
			qfaufliaMust(t, translated.Cleanup())
			if proofs {
				proof, err := target.RefGet(pr)
				qfaufliaMust(t, err)
				fact, err := target.GetProofFact(proof)
				qfaufliaMust(t, err)
				if !target.IsFalse(fact) {
					t.Fatal("SMT proof route did not preserve a contradiction proof")
				}
			} else {
				dependency := core.Get()
				entries, err := target.DependencyLinearize(dependency, nil)
				qfaufliaMust(t, err)
				if len(entries) != 2 {
					t.Fatal("core route lost original labels", entries)
				}
			}
		})
	}
}

func TestQFLIATacticConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 16384; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			c := qfliaContext(t, "(set-logic QF_LIA)")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			m.Budget().FailAfter(stop)
			r, err := c.NewBuiltinTactic("qflia")
			m.Budget().FailAfter(-1)
			if r != nil {
				qfaufliaMust(t, r.IncRef())
				qfaufliaMust(t, r.DecRef())
			}
			var failed memory.ConstructionFailure
			if errors.As(err, &failed) {
				qfaufliaMust(t, failed.Cleanup())
			}
			var memout memory.Failure
			if err == nil {
				success = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &memout) && memout.Kind == memory.FailureMemout) {
				t.Fatal(err)
			}
		})
		if success {
			break
		}
	}
	if !success {
		t.Fatal("no successful factory reservation frontier")
	}
}

func TestQFLIAQuasiPBProbeCPP(t *testing.T) {
	for _, tc := range []struct {
		name, declarations, assertions string
		want                           float64
	}{
		{"empty", "", "", 1},
		{"unbounded", `(declare-const x Int)`, `(assert (>= x 0))`, 0},
		{"binary", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x 0) (<= x 1) (>= y 0) (<= y 1)))`, 1},
		{"one-wide", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x -100) (<= x 100) (>= y 0) (<= y 1)))`, 1},
		{"two-wide", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x -100) (<= x 100) (>= y 0) (<= y 2)))`, 0},
		{"strict-integer", `(declare-const x Int)(declare-const y Int)`, `(assert (and (> x -1) (< x 2) (> y -1) (< y 2)))`, 1},
		{"reversed-zero-one", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x 1) (<= x 0) (>= y 1) (<= y 0)))`, 1},
		{"one-wide-and-unbounded", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x -10) (<= x 10) (>= y 0)))`, 0},
		{"Boolean", `(declare-const p Bool)`, `(assert p)`, 1},
		{"real-two-wide", `(declare-const x Real)(declare-const y Real)`, `(assert (and (>= x 0.0) (<= x 2.0) (>= y 0.0) (<= y 2.0)))`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := `(set-logic ALL)` + tc.declarations + tc.assertions
			c := qfliaContext(t, script)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			globals, err := c.GlobalParameters()
			qfaufliaMust(t, err)
			p, err := smtlogics.NewIsQuasiPBProbe(m, globals.Context, globals)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, p.Close()) })
			goal, err := tactic.NewGoal(m, false, false, false)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
			qfaufliaMust(t, c.AssertGoal(goal))
			value, err := p.Eval(goal)
			qfaufliaMust(t, err)
			if value != tc.want {
				t.Fatal(value, tc.want)
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				command := exec.Command(reference, "-in")
				command.Stdin = strings.NewReader(script + `(apply (if is-quasi-pb fail skip))`)
				cpp, err := command.CombinedOutput()
				if err != nil && !strings.Contains(string(cpp), "tactic failed: fail tactic") {
					t.Fatal(err, string(cpp))
				}
				if strings.Contains(string(cpp), "unknown probe") || strings.Contains(string(cpp), "tactic failed: fail tactic") != (value != 0) {
					t.Fatalf("probe %v: %s", value, cpp)
				}
			}
		})
	}
}

func TestQFLIATacticCPP(t *testing.T) {
	cases := []struct{ name, options, decls, assertions, query string }{
		{"shifted-model", "", `(declare-const x Int)`, `(assert (>= x 5))(assert (<= x 7))(assert (not (= x 5)))(assert (not (= x 6)))`, `(get-value (x))`},
		{"unbounded-ilp", "", `(declare-const x Int)(declare-const y Int)`, `(assert (>= (+ x y) 1))(assert (<= (+ (* 2 x) y) 3))(assert (>= (+ x (* 2 y)) 0))`, `(get-value ((>= (+ x y) 1) (<= (+ (* 2 x) y) 3) (>= (+ x (* 2 y)) 0)))`},
		{"pseudo-Boolean", "", `(declare-const x Int)(declare-const y Int)(declare-const z Int)`, `(assert (and (>= x 0) (<= x 1) (>= y 0) (<= y 1) (>= z 0) (<= z 1)))(assert (>= (+ (* 2 x) (* 3 y) (* 5 z)) 7))(assert (not (= x y)))`, `(get-value ((>= (+ (* 2 x) (* 3 y) (* 5 z)) 7) (not (= x y))))`},
		{"quasi-pb", "", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x 0) (<= x 100) (>= y 0) (<= y 1)))(assert (or (= x 20) (= x 90)))(assert (>= (+ x (* 3 y)) 92))`, `(get-value (x y))`},
		{"bounded", "", `(declare-const x Int)(declare-const y Int)`, `(assert (and (>= x 100) (<= x 110) (>= y 100) (<= y 110)))(assert (or (>= (+ x y) 219) (<= (+ x y) 201)))(assert (not (= x y)))`, `(get-value ((not (= x y)) (or (>= (+ x y) 219) (<= (+ x y) 201))))`},
		{"unbounded-disjunction", "", `(declare-const x Int)(declare-const y Int)`, `(assert (or (>= (+ x y) 4) (<= (+ x y) 0)))(assert (not (= x y)))(assert (>= (+ (* 2 x) y) 2))`, `(get-value ((not (= x y)) (>= (+ (* 2 x) y) 2)))`},
		{"proof-guard", `(set-option :produce-proofs true)`, `(declare-const x Int)(declare-const y Int)`, `(assert (>= (+ x y) 2))(assert (<= (+ x y) 1))`, ""},
		{"core-guard", `(set-option :produce-unsat-cores true)`, `(declare-const x Int)(declare-const y Int)`, `(assert (! (>= (+ x y) 2) :named lo))(assert (! (<= (+ x y) 1) :named hi))`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := `(set-logic QF_LIA)(set-option :produce-models true)` + tc.options + tc.decls + tc.assertions + `(check-sat-using qflia)` + tc.query
			c, err := cmd_context.NewStandaloneContext()
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, c.Close()) })
			var output bytes.Buffer
			qfaufliaMust(t, smt2.ParseCommands(c, strings.NewReader(script), &output))
			if strings.Contains(output.String(), "unknown") || !strings.Contains(output.String(), "sat") {
				t.Fatal(output.String())
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				command := exec.Command(reference, "-in")
				command.Stdin = strings.NewReader(script)
				cpp, err := command.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(cpp))
				}
				if strings.Join(strings.Fields(output.String()), " ") != strings.Join(strings.Fields(string(cpp)), " ") {
					t.Fatalf("Go: %s\nC++: %s", output.String(), cpp)
				}
			}
		})
	}
}
