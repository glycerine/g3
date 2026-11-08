package smtlogics_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestQuantifiedArithmeticModelsSurviveStrategyClose(t *testing.T) {
	for _, name := range []string{"uflra", "auflia", "auflira", "aufnira", "ufnia"} {
		t.Run(name, func(t *testing.T) {
			c := qfliaContext(t, `(set-logic ALL)(declare-const x Int)(declare-fun f (Int) Int)(assert (forall ((t Int)) (>= (f t) t)))(assert (= x (f 3)))(assert (<= x 5))`)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			config := ackrConfig(t, c)
			r, err := c.NewBuiltinTactic(name)
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
				t.Fatal("missing quantified strategy model")
			}
			qfaufliaMust(t, r.Cleanup())
			qfaufliaMust(t, r.DecRef())
			closed = true
			var out tactic.ModelRef
			t.Cleanup(func() { qfaufliaMust(t, out.Close()) })
			qfaufliaMust(t, tactic.ModelConverterToModel(m, result.ModelConverter, &out))
			md, err := out.Get()
			qfaufliaMust(t, err)
			ev, err := model.NewModelEvaluator(md, config.ModelEvaluator)
			qfaufliaMust(t, err)
			t.Cleanup(func() { qfaufliaMust(t, ev.Close()) })
			for i := uint32(1); i < 3; i++ {
				h, err := original.Form(i)
				qfaufliaMust(t, err)
				value, err := ev.EvaluateValue(h)
				qfaufliaMust(t, err)
				v, err := m.RefGet(value)
				qfaufliaMust(t, err)
				valid := m.IsTrue(v)
				qfaufliaMust(t, m.DelRef(&value))
				if !valid {
					t.Fatal("model lost original ground assertion", i)
				}
			}
		})
	}
}

func TestQuantifiedArithmeticAllocationFrontiers(t *testing.T) {
	for _, name := range []string{"uflra", "auflia", "ufnia"} {
		for _, phase := range []string{"constructor", "apply"} {
			t.Run(name+"/"+phase, func(t *testing.T) {
				success := false
				for stop := int64(0); stop < 8192 && !success; stop++ {
					t.Run(fmt.Sprint(stop), func(t *testing.T) {
						script := `(set-logic ALL)(declare-const x Int)(declare-fun f (Int) Int)(assert (>= x 0))(assert (<= x 3))(assert (>= (f x) 1))(assert (<= (f x) 2))`
						if name == "ufnia" {
							// Exercise QE traversal and nonlinear SMT, including owned
							// quantifier state on every injected apply failure.
							script = `(set-logic ALL)(declare-const x Int)(declare-fun f (Int) Int)(assert (forall ((t Int)) (! (>= (f t) (* t t)) :pattern ((f t)))))(assert (= x (f 3)))(assert (<= x 10))`
						}
						c := qfliaContext(t, script)
						m, err := c.Manager()
						qfaufliaMust(t, err)
						var r tactic.Tactic
						var goal *tactic.Goal
						var result tactic.TacticResult
						if phase == "apply" {
							r, err = c.NewBuiltinTactic(name)
							qfaufliaMust(t, err)
							qfaufliaMust(t, r.IncRef())
							t.Cleanup(func() { qfaufliaMust(t, r.DecRef()) })
							goal, err = tactic.NewGoal(m, false, true, false)
							qfaufliaMust(t, err)
							t.Cleanup(func() { qfaufliaMust(t, goal.Close()) })
							qfaufliaMust(t, c.AssertGoal(goal))
							t.Cleanup(func() { qfaufliaMust(t, result.Close()) })
						}
						m.Budget().FailAfter(stop)
						if phase == "constructor" {
							r, err = c.NewBuiltinTactic(name)
						} else {
							err = r.Apply(goal, &result)
						}
						m.Budget().FailAfter(-1)
						if phase == "constructor" && r != nil {
							qfaufliaMust(t, r.IncRef())
							qfaufliaMust(t, r.DecRef())
						}
						var failure memory.ConstructionFailure
						if errors.As(err, &failure) {
							qfaufliaMust(t, failure.Cleanup())
						}
						if err == nil {
							success = true
							if phase == "apply" && !result.DecidedSat() {
								t.Fatal("strategy did not solve fixture")
							}
						} else if !errors.Is(err, arena.ErrInjectedFailure) {
							var memout memory.Failure
							if !errors.As(err, &memout) || memout.Kind != memory.FailureMemout {
								t.Fatal(err)
							}
						}
					})
					if t.Failed() {
						t.Fatal("quantified tactic allocation frontier", stop)
					}
				}
				if !success {
					t.Fatal("no successful frontier")
				}
			})
		}
	}
}

func TestAUFLIAQICostTraceAndClosedOutput(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(fmt.Sprint(closed), func(t *testing.T) {
			c := qfliaContext(t, `(set-logic ALL)`)
			m, err := c.Manager()
			qfaufliaMust(t, err)
			config := ackrConfig(t, c)
			var out bytes.Buffer
			var writer io.Writer = &out
			if closed {
				writer = closedAckrTraceWriter{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			tags.Enable("qi_cost")
			t.Cleanup(tags.Finalize)
			config.ModelEvaluator.Trace = tags
			r, err := smtlogics.NewAUFLIATactic(m, nil, config)
			qfaufliaMust(t, err)
			qfaufliaMust(t, r.IncRef())
			qfaufliaMust(t, r.DecRef())
			if !closed && tags.ShouldTrace("qi_cost") {
				if !strings.Contains(out.String(), "src/tactic/smtlogics/quant_tactics.cpp:82") || !strings.Contains(out.String(), "(params qi.cost 0)\n0\n") {
					t.Fatal(out.String())
				}
			}
		})
	}
}

func TestQuantifiedArithmeticTranslatedProofAndCore(t *testing.T) {
	for _, name := range []string{"uflra", "auflia", "auflira", "aufnira", "ufnia"} {
		modes := []bool{false, true}
		if name == "ufnia" {
			// Source QE-lite dereferences an empty proof on quantified
			// proof-enabled goals. Its native refusal is tested in qe.
			modes = []bool{false}
		}
		for _, proofs := range modes {
			t.Run(fmt.Sprintf("%s/proofs=%v", name, proofs), func(t *testing.T) {
				option := "produce-unsat-cores"
				mode := ast.ASTProofDisabled
				if proofs {
					option = "produce-proofs"
					mode = ast.ASTProofEnabled
				}
				c := qfliaContext(t, "(set-logic ALL)(set-option :"+option+" true)(declare-fun f (Int) Int)(assert (! (forall ((x Int)) (! (>= (f x) x) :pattern ((f x)))) :named axiom))(assert (! (< (f 0) 0) :named negative))")
				m, err := c.Manager()
				qfaufliaMust(t, err)
				r, err := c.NewBuiltinTactic(name)
				qfaufliaMust(t, err)
				qfaufliaMust(t, r.IncRef())
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
				var md tactic.ModelRef
				t.Cleanup(func() { qfaufliaMust(t, md.Close()) })
				var labels tactic.ModelLabels
				t.Cleanup(labels.Close)
				pr, err := target.NewRef(ast.ASTHandle{})
				qfaufliaMust(t, err)
				t.Cleanup(func() { qfaufliaMust(t, target.DelRef(&pr)) })
				var dependency tactic.DependencyRef
				t.Cleanup(func() { qfaufliaMust(t, dependency.Close()) })
				status, reason, err := tactic.CheckSat(translated, copy, &md, &labels, pr, &dependency)
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
						t.Fatal("quantified contradiction proof lost")
					}
				} else {
					entries, err := target.DependencyLinearize(dependency.Get(), nil)
					qfaufliaMust(t, err)
					if len(entries) != 2 {
						t.Fatal("quantified core lost source labels", entries)
					}
				}
			})
		}
	}
}
