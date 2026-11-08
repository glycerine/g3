package smtlogics_test

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"strings"
	"testing"
)

func TestQFUFBVTacticModelSurvivesStrategyClose(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{"smt", `(set-logic QF_UFBV)(set-option :produce-models true)
(declare-fun f ((_ BitVec 4) (_ BitVec 4)) (_ BitVec 4))
(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))
(assert (bvult x y))(assert (bvult (f #x1 x) (f #x1 y)))`},
		{"sat", `(set-logic QF_UFBV)(set-option :produce-models true)
(declare-fun f ((_ BitVec 4)) (_ BitVec 4))
(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))
(assert (= (bvmul x y) #x6))(assert (bvult x y))(assert (= (f #x0) #x1))`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qfliaContext(t, tc.script)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			r, err := c.NewBuiltinTactic("qfufbv")
			qfaufliaMust(t, err)
			qfaufliaMust(t, r.IncRef())
			closed := false
			t.Cleanup(func() {
				if !closed {
					qfaufliaMust(t, r.DecRef())
				}
			})
			goal, err := tactic.NewGoal(m, false, true, false)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
			qfaufliaMust(t, c.AssertGoal(goal))
			original, err := tactic.NewGoalCopy(goal)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, original.Close()) })
			var result tactic.TacticResult
			t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
			qfaufliaMust(t, r.Apply(goal, &result))
			if !result.DecidedSat() || result.ModelConverter == nil {
				t.Fatal("missing reconstructable model")
			}
			qfaufliaMust(t, r.Cleanup())
			qfaufliaMust(t, r.DecRef())
			closed = true
			var reference tactic.ModelRef
			t.Cleanup(func() { qfaufliaMust(t, reference.Close()) })
			qfaufliaMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &reference))
			md, err := reference.Get()
			qfaufliaMust(t, err)
			globals, err := c.GlobalParameters()
			qfaufliaMust(t, err)
			coop, err := cooperate.NewCooperation(m.Budget())
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, coop.Close()) })
			evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: globals.Context, Globals: globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: coop})
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, evaluator.Close()) })
			qfaufliaMust(t, evaluator.SetModelCompletion(true))
			size, err := original.Size()
			qfaufliaMust(t, err)
			if size == 0 {
				t.Fatal("original model check is empty")
			}
			for i := uint32(0); i < size; i++ {
				formula, err := original.Form(i)
				qfaufliaMust(t, err)
				value, err := evaluator.EvaluateValue(formula)
				qfaufliaMust(t, err)
				h, err := m.RefGet(value)
				qfaufliaMust(t, err)
				valid := m.IsTrue(h)
				qfaufliaMust(t, m.DelRef(&value))
				if !valid {
					t.Fatalf("original formula %d is false in the reconstructed model", i)
				}
			}
			var declarations []ast.ASTHandle
			n, err := md.NumConstants()
			qfaufliaMust(t, err)
			for i := uint32(0); i < n; i++ {
				d, err := md.Constant(i)
				qfaufliaMust(t, err)
				declarations = append(declarations, d)
			}
			for i := uint32(0); i < md.NumFunctionDeclarations(); i++ {
				d, err := md.Function(i)
				qfaufliaMust(t, err)
				declarations = append(declarations, d)
			}
			for _, d := range declarations {
				name, err := m.GetASTName(d)
				qfaufliaMust(t, err)
				text, err := m.Symbols.String(name)
				qfaufliaMust(t, err)
				if strings.Contains(text, "!") {
					t.Fatal("fresh declaration escaped model filtering", text)
				}
			}
		})
	}
}

func TestQFUFBVTacticConstructorAllocationFrontiers(t *testing.T) {
	success := false
	for stop := int64(0); stop < 8192 && !success; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			c := qfliaContext(t, `(set-logic ALL)`)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			m.Budget().FailAfter(stop)
			r, err := c.NewBuiltinTactic("qfufbv")
			m.Budget().FailAfter(-1)
			if r != nil {
				qfaufliaMust(t, r.IncRef())
				qfaufliaMust(t, r.DecRef())
			}
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				qfaufliaMust(t, failure.Cleanup())
			}
			var memout memory.Failure
			if err == nil {
				success = true
			} else if !errors.Is(err, arena.ErrInjectedFailure) && !(errors.As(err, &memout) && memout.Kind == memory.FailureMemout) {
				t.Fatal(err)
			}
		})
	}
	if !success {
		t.Fatal("no successful factory reservation frontier")
	}
}

func TestQFUFBVTacticTranslatedProofAndCoreRoutes(t *testing.T) {
	for _, proofs := range []bool{false, true} {
		t.Run(fmt.Sprint(proofs), func(t *testing.T) {
			option := "produce-unsat-cores"
			if proofs {
				option = "produce-proofs"
			}
			c := qfliaContext(t, "(set-logic QF_UFBV)(set-option :"+option+" true)(declare-fun f ((_ BitVec 4)) (_ BitVec 4))(declare-const x (_ BitVec 4))(declare-const y (_ BitVec 4))(assert (! (and (bvult (f x) (f y)) (bvult (f y) (f x))) :named impossible))")
			m, err := c.Manager()
			qfaufliaMust(t, err)
			r, err := c.NewBuiltinTactic("qfufbv")
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
				if len(entries) != 1 {
					t.Fatal("core route lost original labels", entries)
				}
			}
		})
	}
}
