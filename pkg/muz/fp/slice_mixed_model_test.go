package fp

// Sources: src/muz/spacer/spacer_context.cpp init_rules/get_model and
// src/muz/spacer/spacer_dl_interface.cpp check_reset. Reconstructed models
// must satisfy original clauses and refute the query after every preprocessing run.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSliceMixedSortModelSatisfiesOriginalClauses(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
			}
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "true"))
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			fragmentMust(t, e.SelectEngine(backend))
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
			zero := own(m.ArithmeticIntegerNumeralText("0"))
			one := own(m.ArithmeticRealNumeralText("1"))
			realZero := own(m.ArithmeticRealNumeralText("0"))
			integer, err := m.GetSort(zero)
			fragmentMust(t, err)
			realSort, err := m.GetSort(one)
			fragmentMust(t, err)
			domain := []ast.ASTHandle{boolean, integer, integer, realSort}
			variables := make([]ast.ASTHandle, len(domain))
			for i, sort := range domain {
				variables[i] = own(m.MkVar(uint32(i), sort))
			}
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			predicate := decl("mixed-slice-P", domain)
			goal := decl("mixed-slice-Goal", nil)
			app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
			var originals []ast.ASTHandle
			add := func(formula ast.ASTHandle, name string) {
				sorts := []ast.ASTHandle{realSort, integer, integer, boolean}
				names := []symbol.Symbol{symbol.Numerical(3), symbol.Numerical(2), symbol.Numerical(1), symbol.Numerical(0)}
				formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
				originals = append(originals, formula)
				fragmentMust(t, e.addRuleEager(formula, name))
			}
			add(app(predicate, variables[0], zero, variables[2], one), "mixed-fact")
			negative := own(m.MkOrArray([]ast.ASTHandle{own(m.MkArithmeticLT(variables[1], zero)), own(m.MkArithmeticLT(variables[3], realZero))}))
			body := own(m.MkAndArray([]ast.ASTHandle{app(predicate, variables...), negative}))
			add(own(m.MkImplies(body, app(goal))), "mixed-query")
			cooperation, err := cooperate.NewCooperation(m.Budget())
			fragmentMust(t, err)
			defer func() { fragmentMust(t, cooperation.Close()) }()
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, solver.Close()) }()
			for repeat := 0; repeat < 2; repeat++ {
				// Direct PDR predicate models deliberately omit their query root.
				// An expression query supplies a synthetic root, so the original
				// Goal remains subject to this model-refutation check.
				status, err := e.QueryExpression(app(goal))
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("negative integer should be unreachable", status)
				}
				if e.queryClone == nil {
					t.Fatal("expression query did not create child engine")
				}
				columns := e.queryClone.spacerSlices
				if backend == "pdr" {
					columns = e.queryClone.pdrSlices
				}
				removed := columns[predicate]
				if len(removed) != 4 || !removed[0] || removed[1] || !removed[2] || removed[3] {
					t.Fatal("expected nonadjacent mixed-sort columns removed", removed)
				}

				md, err := e.ExpressionModel()
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
					t.Fatal("expanded model does not refute query", status)
				}
				fragmentMust(t, solver.Pop(1))
				fragmentMust(t, m.DelRef(&value))
				fragmentMust(t, evaluator.Close())
				fragmentMust(t, md.Close())
			}

		})
	}
}
