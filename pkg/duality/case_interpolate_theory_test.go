package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityCaseInterpolationUsesBackgroundTheory(t *testing.T) {
	ctx, m, main, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{
		Params: ctx.config.params, Globals: globals,
	})
	must(err)
	defer func() { must(factory.Close()) }()
	auxiliary, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer func() { must(auxiliary.Close()) }()
	checker, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer func() { must(checker.Close()) }()
	services := derivationEvaluatorFixture(t, ctx, globals)
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer func() { must(simplifier.Close()) }()
	quantifiers, err := qe.NewLite(m, nil, true, qe.Config{
		Context: ctx.config.params, Globals: globals,
		Cooperation: services.Cooperation,
	})
	must(err)
	defer func() { must(quantifiers.Close()) }()

	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(main))
	must(g.BindModelEvaluator(services))
	p, err := ctx.BoolConst("case-theory-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("case-theory-q")
	must(err)
	closeExpr(t, q)
	for _, term := range []*Expr{p, q} {
		declaration, err := term.Decl()
		must(err)
		must(g.DeclareConstant(declaration))
		must(declaration.Close())
	}
	bridge, err := q.Implies(p)
	must(err)
	closeExpr(t, bridge)
	bridgeHandle, err := bridge.Raw()
	must(err)
	must(main.Push())
	must(main.Assert(bridgeHandle))
	must(auxiliary.Push())
	must(auxiliary.Assert(bridgeHandle))
	must(checker.Push())
	must(checker.Assert(bridgeHandle))

	parentTerm, err := ctx.BoolConst("case-theory-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("case-theory-child")
	must(err)
	closeExpr(t, childTerm)
	parent, err := g.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := g.CreateNode(childTerm)
	must(err)
	negativeP, err := p.Not()
	must(err)
	closeExpr(t, negativeP)
	parentBody, err := negativeP.And(childTerm)
	must(err)
	closeExpr(t, parentBody)
	childDeclaration, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDeclaration)
	parentRule, err := NewTransformer(ctx, []*FuncDecl{childDeclaration}, nil, parentBody)
	must(err)
	defer func() { must(parentRule.Close()) }()
	childRule, err := NewTransformer(ctx, nil, nil, q)
	must(err)
	defer func() { must(childRule.Close()) }()
	pFact, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer func() { must(pFact.Close()) }()
	parentEdge, err := g.CreateEdge(parent, parentRule, []*Node{child})
	must(err)
	childEdge, err := g.CreateEdge(child, childRule, nil)
	must(err)
	must(g.Push())
	must(g.AssertNode(parent))
	must(g.AssertEdge(parentEdge, 0, false, false))
	must(g.AssertEdge(childEdge, 0, false, false))
	warmStatus, err := g.Check(parent, nil, nil)
	must(err)
	if warmStatus != lbool.False {
		t.Fatal("bridge-dependent case context was not UNSAT", warmStatus)
	}
	mainDepth, err := main.ScopeLevel()
	must(err)
	auxDepth, err := auxiliary.ScopeLevel()
	must(err)
	mainAssertions, err := main.NumAssertions()
	must(err)
	auxAssertions, err := auxiliary.NumAssertions()
	must(err)

	runner := &CaseInterpolator{
		Graph: g, Auxiliary: auxiliary, Extensional: true,
		Simplifier: simplifier, Quantifiers: quantifiers,
		Theory: []*Expr{bridge},
	}
	must(runner.Interpolate(parent, child, nil))

	qImpliesLearned, err := childRule.SubsetEq(child.Annotation, checker)
	must(err)
	learnedImpliesP, err := child.Annotation.SubsetEq(pFact, checker)
	must(err)
	if !qImpliesLearned || !learnedImpliesP {
		t.Fatal("case interpolant does not separate q from not-p", qImpliesLearned, learnedImpliesP)
	}
	gotMainDepth, err := main.ScopeLevel()
	must(err)
	gotAuxDepth, err := auxiliary.ScopeLevel()
	must(err)
	gotMainAssertions, err := main.NumAssertions()
	must(err)
	gotAuxAssertions, err := auxiliary.NumAssertions()
	must(err)
	if gotMainDepth != mainDepth || gotAuxDepth != auxDepth ||
		gotMainAssertions != mainAssertions || gotAuxAssertions != auxAssertions ||
		len(childEdge.constraints) != 0 {
		t.Fatal("case interpolation did not restore solver scopes",
			gotMainDepth, mainDepth, gotAuxDepth, auxDepth,
			gotMainAssertions, mainAssertions, gotAuxAssertions, auxAssertions,
			len(childEdge.constraints))
	}

	must(g.Pop(1))
	must(checker.Pop(1))
	must(auxiliary.Pop(1))
	must(main.Pop(1))
}
