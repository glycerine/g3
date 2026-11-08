package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::query slicing stage.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestBMCPreprocessingStillSlicesUnusedColumns(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	for _, key := range []string{"inline_eager", "inline_linear", "subsumption_checker"} {
		fragmentMust(t, g.SourceSetName("fixedpoint.xform."+key, "false"))
	}
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.slice", "true"))
	fragmentMust(t, g.SourceSetName("fixedpoint.xform.unfold_rules", "2"))
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, e.Close()) }()
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
	goal := decl("stage-goal", nil)
	px := own(m.MkApp(p, []ast.ASTHandle{x}))
	goalExpr := own(m.MkApp(goal, nil))
	forall := func(body ast.ASTHandle) ast.ASTHandle {
		return own(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: body}))
	}
	fragmentMust(t, e.addRuleEager(forall(px), "base"))
	fragmentMust(t, e.addRuleEager(forall(own(m.MkImplies(px, goalExpr))), "goal"))
	prepared, err := e.preprocessHornRulesWithStages(goal, nil, true, true, false, true, false)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, prepared.Close()) }()
	sliced, ok := prepared.slicedPredicates[p]
	if !ok || sliced == p {
		t.Fatal("BMC preprocessing did not slice the unused P column")
	}
	arity, err := m.GetArity(sliced)
	fragmentMust(t, err)
	if arity != 0 {
		t.Fatalf("sliced P arity = %d, want 0", arity)
	}
}
