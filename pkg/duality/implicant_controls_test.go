package duality

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestRPFPFixCurrentStateDoesNotFreezeUnderapproximationControls(t *testing.T) {
	ctx, _, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	must(g.BindSolver(native))
	must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	p, err := ctx.BoolConst("fixed-control-p")
	must(err)
	closeExpr(t, p)
	pd, err := p.Decl()
	must(err)
	defer pd.Close()
	must(g.DeclareConstant(pd))
	parentTerm, err := ctx.BoolConst("fixed-control-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("fixed-control-child")
	must(err)
	closeExpr(t, childTerm)
	cd, err := childTerm.Decl()
	must(err)
	defer cd.Close()
	parent, err := g.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := g.CreateNode(childTerm)
	must(err)
	formula, err := p.Or(childTerm)
	must(err)
	closeExpr(t, formula)
	rule, err := NewTransformer(ctx, []*FuncDecl{cd}, nil, formula)
	must(err)
	defer rule.Close()
	edge, err := g.CreateEdge(parent, rule, []*Node{child})
	must(err)
	must(g.Push())
	must(g.AssertNode(parent))
	must(g.AssertEdge(edge, 0, false, true))
	flag, err := g.UnderapproxFlag(child)
	must(err)
	defer flag.Close()
	negativeFlag, err := flag.Not()
	must(err)
	defer negativeFlag.Close()
	activation, variables, err := g.RedVars(child)
	must(err)
	defer activation.Close()
	for _, v := range variables {
		defer v.Close()
	}
	inactive, err := activation.Not()
	must(err)
	defer inactive.Close()
	// Select a model with an inactive child and false optional underapprox flag,
	// then remove only those temporary model-selection assertions.
	must(native.Push())
	for _, term := range []*Expr{p, negativeFlag, inactive} {
		h, err := term.Raw()
		must(err)
		must(native.Assert(h))
	}
	status, err := g.Check(parent, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	must(native.Pop(1))
	profile := NewProfiler(nil)
	ctx.activeProfiler = profile
	must(g.FixCurrentState(edge))
	if profile.top.children["UnderapproxFormula"] == nil || profile.current != profile.top {
		t.Fatal("fixed-state underapproximation timer not restored")
	}
	status, err = g.Check(parent, []*Node{child}, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("fixed decisions contradicted the next underapproximation selection", status)
	}
	must(g.Pop(1))
}
