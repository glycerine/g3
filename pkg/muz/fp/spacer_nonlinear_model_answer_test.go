package fp

// Sources: spacer_context.cpp get_model/mk_unsat_answer and
// spacer_dl_interface.cpp check_reset. A nonlinear join of two shared recursive
// instances needs both numeric invariants; model and answer exports must preserve
// all original clauses and query refutation after slicing/inlining and reuse.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerNonlinearSharedModelAndAnswerOriginalClauses(t *testing.T) {
	modes := []string{"plain", "slice", "inline", "slice-inline"}
	for _, name := range modes {
		sliced := name == "slice" || name == "slice-inline"
		t.Run(name, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
			}
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
			if sliced {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "true"))
			}
			if name == "inline" || name == "slice-inline" {
				for _, key := range []string{"inline_eager", "inline_linear"} {
					fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "true"))
				}
			}
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			fragmentMust(t, e.SelectEngine("spacer"))
			var pins []ast.ASTRef
			defer func() {
				for i := len(pins) - 1; i >= 0; i-- {
					fragmentMust(t, m.DelRef(&pins[i]))
				}
			}()
			own := func(h ast.ASTHandle, err error) ast.ASTHandle {
				fragmentMust(t, err)
				r, err := m.NewRef(h)
				fragmentMust(t, err)
				pins = append(pins, r)
				return h
			}
			boolean := own(m.MkBoolSort())
			one := own(m.ArithmeticIntegerNumeralText("1"))
			zero := own(m.ArithmeticIntegerNumeralText("0"))
			five := own(m.ArithmeticIntegerNumeralText("5"))
			integer, err := m.GetSort(one)
			fragmentMust(t, err)
			x := own(m.MkVar(0, integer))
			y := own(m.MkVar(1, integer))
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			goal := decl("numeric-goal", nil)
			left := decl("numeric-left", []ast.ASTHandle{integer})
			right := decl("numeric-right", []ast.ASTHandle{integer})
			leaf := decl("numeric-leaf", []ast.ASTHandle{integer, integer})
			app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
			and := func(args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkAndArray(args)) }
			var originals []ast.ASTHandle
			add := func(body, head ast.ASTHandle, n int, name string) {
				formula := own(m.MkImplies(body, head))
				sorts := make([]ast.ASTHandle, n)
				names := make([]symbol.Symbol, n)
				for i := range sorts {
					sorts[i] = integer
					names[i] = symbol.Numerical(uint32(i))
				}
				if n != 0 {
					formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
				}
				originals = append(originals, formula)
				fragmentMust(t, e.AddRule(formula, name))
			}
			add(own(m.MkTrue()), app(leaf, zero, y), 2, "model-base")
			add(and(app(leaf, x, y), own(m.MkArithmeticLT(x, five))), app(leaf, own(m.MkArithmeticAdd(x, one)), y), 2, "model-successor")
			add(app(leaf, x, y), app(left, x), 2, "left-projection")
			add(app(leaf, x, y), app(right, x), 2, "right-projection")
			add(and(app(left, x), app(right, y), own(m.MkArithmeticLT(own(m.MkArithmeticAdd(x, y)), zero))), app(goal), 2, "nonlinear-negative-sum")
			cooperation, err := cooperate.NewCooperation(m.Budget())
			fragmentMust(t, err)
			defer func() { fragmentMust(t, cooperation.Close()) }()
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, solver.Close()) }()
			for repeat := 0; repeat < 2; repeat++ {
				status, err := e.QueryPredicate(goal)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("negative integer should be unreachable", status)
				}
				if sliced && !slicedPredicateHasRemovedColumns(e.spacerSlices, leaf) {
					t.Fatal("fixture did not exercise removed numeric relation column")
				}
				if name == "inline" || name == "slice-inline" {
					for _, intermediate := range []ast.ASTHandle{left, right} {
						eliminated, err := e.hornInlineEliminated(e.spacerConversions, intermediate)
						fragmentMust(t, err)
						if !eliminated {
							t.Fatal("fixture did not inline intermediate relation", intermediate)
						}
					}
				}
				// Eager inlining may move the nonlinear rule onto the synthetic
				// query predicate. Require a live nonlinear rule in either position.
				nonlinear := false
				for _, declaration := range []ast.ASTHandle{e.spacerPredicate(goal), e.spacerQuery} {
					transformer, found, err := e.spacer.Transformer(declaration)
					fragmentMust(t, err)
					if !found {
						continue
					}
					rules, err := transformer.Rules()
					fragmentMust(t, err)
					for _, rule := range rules {
						tails, err := rule.UninterpretedTailSize()
						fragmentMust(t, err)
						nonlinear = nonlinear || tails == 2
						fragmentMust(t, rule.Close())
					}
				}
				if !nonlinear {
					t.Fatal("fixture did not retain nonlinear solver rule")
				}
				md, err := e.Model(goal)
				fragmentMust(t, err)
				evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: g.Context, Globals: g, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
				fragmentMust(t, err)
				fragmentMust(t, evaluator.SetModelCompletion(false))
				for i, formula := range originals {
					value, err := evaluator.EvaluateValue(formula)
					fragmentMust(t, err)
					h, err := m.RefGet(value)
					fragmentMust(t, err)
					fragmentMust(t, solver.Push())
					fragmentMust(t, solver.Assert(own(m.MkNot(h))))
					status, err := solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.False {
						t.Fatal("exported model does not satisfy original clause", i, status)
					}
					fragmentMust(t, solver.Pop(1))
					fragmentMust(t, m.DelRef(&value))
				}
				value, err := evaluator.EvaluateValue(app(goal))
				fragmentMust(t, err)
				h, err := m.RefGet(value)
				fragmentMust(t, err)
				fragmentMust(t, solver.Push())
				fragmentMust(t, solver.Assert(h))
				status, err = solver.CheckSat(nil)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("exported model does not refute nonlinear query", status)
				}
				fragmentMust(t, solver.Pop(1))
				fragmentMust(t, m.DelRef(&value))
				fragmentMust(t, evaluator.Close())
				fragmentMust(t, md.Close())
				answer, err := e.Answer(goal)
				fragmentMust(t, err)
				answerFormula, err := m.RefGet(answer)
				fragmentMust(t, err)
				fragmentMust(t, solver.Push())
				fragmentMust(t, solver.Assert(answerFormula))
				status, err = solver.CheckSat(nil)
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("converted answer is inconsistent", status)
				}
				obligations := append(append([]ast.ASTHandle{}, originals...), own(m.MkNot(app(goal))))
				for i, formula := range obligations {
					fragmentMust(t, solver.Push())
					fragmentMust(t, solver.Assert(own(m.MkNot(formula))))
					status, err = solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.False {
						t.Fatal("answer does not enforce original clause/query", i, status)
					}
					fragmentMust(t, solver.Pop(1))
				}
				fragmentMust(t, solver.Pop(1))
				fragmentMust(t, m.DelRef(&answer))

			}
		})
	}
}
