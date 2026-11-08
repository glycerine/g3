package fp

// Source: PDR/Spacer dl_interface::get_model forwards the active query model.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestExpressionModelOriginalClausesAndLifecycle(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer"} {
		t.Run(backend, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			for _, key := range []string{"inline_eager", "inline_linear", "subsumption_checker"} {
				fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
			}
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "false"))
			fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
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
			integer, err := m.GetSort(zero)
			fragmentMust(t, err)
			x := own(m.MkVar(0, integer))
			decl := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
				s, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d := own(m.MkFuncDecl(s, domain, boolean, ast.ASTFuncDeclInfo{}))
				fragmentMust(t, e.RegisterPredicate(d))
				return d
			}
			p := decl("stage-p", []ast.ASTHandle{integer})
			forall := func(body ast.ASTHandle) ast.ASTHandle {
				return own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
			}
			pzero := own(m.MkApp(p, []ast.ASTHandle{zero}))
			one := own(m.ArithmeticIntegerNumeralText("1"))
			px := own(m.MkApp(p, []ast.ASTHandle{x}))
			next := own(m.MkApp(p, []ast.ASTHandle{own(m.MkArithmeticAdd(x, one))}))
			step := forall(own(m.MkImplies(px, next)))
			originals := []ast.ASTHandle{pzero, step}
			for _, rule := range originals {
				fragmentMust(t, e.addRuleEager(rule, ""))
			}
			status, err := e.QueryExpression(pzero)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("fact query", status)
			}
			if md, err := e.ExpressionModel(); err == nil || md != nil {
				t.Fatal("SAT query published an inductive model", md, err)
			}
			query := own(m.MkAndArgs([]ast.ASTHandle{px, own(m.MkArithmeticLT(x, zero))}))
			cooperation, err := cooperate.NewCooperation(m.Budget())
			fragmentMust(t, err)
			defer func() { fragmentMust(t, cooperation.Close()) }()
			solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
			fragmentMust(t, err)
			defer func() { fragmentMust(t, solver.Close()) }()
			for repeat := 0; repeat < 2; repeat++ {
				status, err := e.QueryExpression(query)
				fragmentMust(t, err)
				if status != lbool.False {
					t.Fatal("negative query", status)
				}
				md, err := e.ExpressionModel()
				fragmentMust(t, err)
				if repeat == 1 {
					fragmentMust(t, e.addRuleEager(pzero, "later"))
					if stale, err := e.ExpressionModel(); err == nil || stale != nil {
						t.Fatal("stale model", stale, err)
					}
					fragmentMust(t, e.Close())
				}
				evaluator, err := model.NewModelEvaluator(md.Core(), model.ModelEvaluatorConfig{Context: g.Context, Globals: g, Memory: m.NumeralService().RuntimeOwner(), Cooperation: cooperation})
				fragmentMust(t, err)
				fragmentMust(t, evaluator.SetModelCompletion(false))
				obligations := append(append([]ast.ASTHandle{}, originals...), forall(own(m.MkNot(query))))
				for i, formula := range obligations {
					value, err := evaluator.EvaluateValue(formula)
					fragmentMust(t, err)
					h, err := m.RefGet(value)
					fragmentMust(t, err)
					fragmentMust(t, solver.Push())
					fragmentMust(t, solver.Assert(own(m.MkNot(h))))
					status, err := solver.CheckSat(nil)
					fragmentMust(t, err)
					if status != lbool.False {
						t.Fatal("model violates original clause/query refutation", i, status)
					}
					fragmentMust(t, solver.Pop(1))
					fragmentMust(t, m.DelRef(&value))
				}
				fragmentMust(t, evaluator.Close())
				fragmentMust(t, md.Close())
			}
		})
	}
}
