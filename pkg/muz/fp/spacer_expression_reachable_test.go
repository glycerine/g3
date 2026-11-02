package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp get_reachable forwards to
// the active query context, including expression queries.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestSpacerExpressionQueryReachableSummary(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	for _, key := range []string{"inline_eager", "inline_linear", "subsumption_checker"} {
		fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
	}
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "false"))
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "0"))
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
	fragmentMust(t, e.AddRule(pzero, "fact"))
	for repeat := 0; repeat < 2; repeat++ {
		status, err := e.QueryExpression(pzero)
		fragmentMust(t, err)
		if status != lbool.True {
			t.Fatal("expression query", status)
		}
		summary, err := e.Reachable(p)
		fragmentMust(t, err)
		if repeat == 1 {
			// Mutation must still reject a stale query, while an already returned
			// summary remains caller-owned after the active query is disposed.
			fragmentMust(t, e.AddRule(pzero, "later-fact"))
			if stale, err := e.Reachable(p); err == nil || stale != (ast.ASTRef{}) {
				t.Fatal("mutation exposed a stale reachable summary", stale, err)
			}
			fragmentMust(t, e.Close())
		}
		h, err := m.RefGet(summary)
		fragmentMust(t, err)
		expected := own(m.MkEq(x, zero))
		equivalent := forall(own(m.MkEq(h, expected)))
		solver, err := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: g.Context, Globals: g})
		fragmentMust(t, err)
		fragmentMust(t, solver.Assert(own(m.MkNot(equivalent))))
		status, err = solver.CheckSat(nil)
		fragmentMust(t, err)
		if status != lbool.False {
			t.Fatal("reach summary is not exactly the original fact", status)
		}
		fragmentMust(t, solver.Close())
		fragmentMust(t, m.DelRef(&summary))
	}
}
