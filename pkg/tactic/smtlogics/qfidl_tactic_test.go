package smtlogics_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func qfidlContext(t *testing.T, script string) *cmd_context.Context {
	t.Helper()
	c, err := cmd_context.NewStandaloneContext()
	qfaufliaMust(t, err)
	t.Cleanup(func() { qfaufliaMust(t, c.Close()) })
	qfaufliaMust(t, smt2.ParseCommands(c, strings.NewReader(script), io.Discard))
	return c
}

func TestQFIDLTacticModelSurvivesStrategyClose(t *testing.T) {
	c := qfidlContext(t, `(set-logic QF_IDL)(set-option :produce-models true)
(declare-const x Int)(assert (>= x 5))(assert (<= x 7))
(assert (not (= x 5)))(assert (not (= x 6)))`)
	m, err := c.Manager()
	qfaufliaMust(t, err)
	x, err := c.MakeApp("x", nil, nil, ast.ASTHandle{})
	qfaufliaMust(t, err)
	r, err := c.NewBuiltinTactic("qfidl")
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

func TestQFIDLTacticTranslatedProofAndCoreRoutes(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			option := "produce-unsat-cores"
			if proofs {
				option = "produce-proofs"
			}
			c := qfidlContext(t, "(set-logic QF_IDL)(set-option :"+option+" true)(declare-const x Int)(assert (! (>= x 3) :named lo))(assert (! (<= x 2) :named hi))")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			r, err := c.NewBuiltinTactic("qfidl")
			qfaufliaMust(t, err)
			qfaufliaMust(t, r.IncRef())
			mode := ast.ASTProofDisabled
			if proofs {
				mode = ast.ASTProofEnabled
			}
			target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget(), ProofMode: mode})
			qfaufliaMust(t, err)
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

func TestQFIDLTacticFiveThousandConstantBoundaryCPP(t *testing.T) {
	for _, count := range []int{4999, 5000, 5001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			c := qfidlContext(t, "(set-logic QF_LIA)(set-option :produce-models false)")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			integer, err := c.LookupSort("Int")
			qfaufliaMust(t, err)
			zero, err := m.ArithmeticIntegerNumeralText("0")
			qfaufliaMust(t, err)
			qfaufliaPin(t, m, zero)
			args := make([]ast.ASTHandle, 0, count)
			var held []ast.ASTHandle
			defer func() {
				for i := len(held) - 1; i >= 0; i-- {
					qfaufliaMust(t, m.DecRef(held[i]))
				}
			}()
			var script strings.Builder
			script.WriteString("(set-logic QF_LIA)(set-option :produce-models false)\n")
			for i := 0; i < count; i++ {
				name := "x" + strconv.Itoa(i)
				symbol, err := m.Symbols.Intern(name)
				qfaufliaMust(t, err)
				x, err := m.MkConst(symbol, integer)
				qfaufliaMust(t, err)
				qfaufliaMust(t, m.IncRef(x))
				held = append(held, x)
				eq, err := m.MkEq(x, zero)
				qfaufliaMust(t, err)
				qfaufliaMust(t, m.IncRef(eq))
				held = append(held, eq)
				args = append(args, eq)
				fmt.Fprintf(&script, "(declare-const %s Int)\n", name)
			}
			f, err := m.MkOrArgs(args)
			qfaufliaMust(t, err)
			g, err := tactic.NewGoal(m, false, false, false)
			qfaufliaMust(t, err)
			defer func() { qfaufliaMust(t, g.Close()) }()
			qfaufliaMust(t, g.Assert(f))
			r, err := c.NewBuiltinTactic("qfidl")
			qfaufliaMust(t, err)
			qfaufliaMust(t, r.IncRef())
			defer func() { qfaufliaMust(t, r.DecRef()) }()
			var result tactic.TacticResult
			defer func() { qfaufliaMust(t, result.Close()) }()
			qfaufliaMust(t, r.Apply(g, &result))
			if !result.DecidedSat() || len(result.Goals) != 1 {
				t.Fatal("boundary goal did not solve", len(result.Goals))
			}
			depth, err := result.Goals[0].Depth()
			qfaufliaMust(t, err)
			want := uint32(0)
			if count < 5000 {
				want = 4
			}
			if depth != want {
				t.Fatal("wrong strategy branch at exact source threshold", depth, want)
			}
			if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
				script.WriteString("(assert (or")
				for i := 0; i < count; i++ {
					fmt.Fprintf(&script, " (= x%d 0)", i)
				}
				script.WriteString("))\n(apply qfidl)\n")
				cmd := exec.Command(reference, "-in")
				cmd.Stdin = strings.NewReader(script.String())
				cpp, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatal(err, string(cpp))
				}
				match := regexp.MustCompile(`:depth ([0-9]+)`).FindSubmatch(cpp)
				if len(match) != 2 || string(match[1]) != strconv.FormatUint(uint64(depth), 10) {
					t.Fatal("source branch/depth mismatch", depth, string(cpp))
				}
			}
		})
	}
}

func TestQFIDLTacticConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 2048; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			c := qfidlContext(t, "(set-logic QF_IDL)")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			m.Budget().FailAfter(stop)
			r, err := c.NewBuiltinTactic("qfidl")
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
