package fp

// Source: src/muz/transforms/dl_mk_scale.cpp and the Spacer query/model
// pipeline. Check the reconstructed model against the unscaled clauses.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerScaleOriginalClauseModel(t *testing.T) {
	t.Helper()
	for _, name := range []string{"disabled", "plain", "slice", "inline", "slice-inline"} {
		sliced := name == "slice" || name == "slice-inline"
		t.Run(name, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			for _, key := range []string{"inline_eager", "inline_linear", "slice", "subsumption_checker", "coalesce_rules"} {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
			}
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
			if name != "disabled" {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform.scale", "true"))
			}
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
			InstallContextModelConverter(e) // a prior context::query installed m_mc
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
			one := own(m.ArithmeticRealNumeralText("1"))
			zero := own(m.ArithmeticRealNumeralText("0"))
			five := own(m.ArithmeticRealNumeralText("5"))
			realSort, err := m.GetSort(one)
			fragmentMust(t, err)
			x := own(m.MkVar(0, realSort))
			y := own(m.MkVar(1, realSort))
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			goal := decl("numeric-goal", nil)
			left := decl("numeric-left", []ast.ASTHandle{realSort})
			leaf := decl("numeric-leaf", []ast.ASTHandle{realSort, realSort})
			app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, args)) }
			and := func(args ...ast.ASTHandle) ast.ASTHandle { return own(m.MkAndArray(args)) }
			var originals []ast.ASTHandle
			add := func(body, head ast.ASTHandle, n int, name string) {
				formula := own(m.MkImplies(body, head))
				sorts := make([]ast.ASTHandle, n)
				names := make([]symbol.Symbol, n)
				for i := range sorts {
					sorts[i] = realSort
					names[i] = symbol.Numerical(uint32(i))
				}
				if n != 0 {
					formula = own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula}))
				}
				originals = append(originals, formula)
				fragmentMust(t, e.addRuleEager(formula, name))
			}
			add(own(m.MkTrue()), app(leaf, zero, y), 2, "model-base")
			add(and(app(leaf, x, y), own(m.MkArithmeticLT(x, five))), app(leaf, own(m.MkArithmeticAdd(x, one)), y), 2, "model-successor")

			add(app(leaf, x, y), app(left, x), 2, "model-projection")
			add(and(app(left, x), own(m.MkArithmeticLT(x, zero))), app(goal), 1, "model-goal")
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
					t.Fatal("negative real should be unreachable", status)
				}
				if name == "disabled" && len(e.spacerScaled) != 0 {
					t.Fatal("scale transform ran with default parameters")
				}
				if name != "disabled" && len(e.spacerScaled) == 0 {
					t.Fatal("scale transform did not record predicates")
				}
				if sliced && !slicedPredicateHasRemovedColumns(e.spacerSlices, e.spacerScaled[leaf]) {
					t.Fatal("fixture did not exercise removed numeric relation column")
				}
				if name == "inline" || name == "slice-inline" {
					eliminated, err := e.hornInlineEliminated(e.spacerConversions, e.spacerScaled[left])
					fragmentMust(t, err)
					if !eliminated {
						t.Fatal("fixture did not inline intermediate relation")
					}
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
					t.Fatal("exported model does not refute query", status)
				}
				fragmentMust(t, solver.Pop(1))
				fragmentMust(t, m.DelRef(&value))
				fragmentMust(t, evaluator.Close())
				fragmentMust(t, md.Close())
			}
		})
	}
}
