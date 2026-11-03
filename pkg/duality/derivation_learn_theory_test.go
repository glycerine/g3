package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDerivationLearningThreadsBackgroundTheoryToInterpolation(t *testing.T) {
	ctx, m, main, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer func() { must(factory.Close()) }()
	aux, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer func() { must(aux.Close()) }()
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer func() { must(manager.Close()) }()

	source, err := NewRPFP(ctx)
	must(err)
	defer func() { must(source.Close()) }()
	p, err := ctx.BoolConst("learning-theory-p")
	must(err)
	closeExpr(t, p)
	pd, err := p.Decl()
	must(err)
	must(source.DeclareConstant(pd))
	must(pd.Close())
	parentTerm, err := ctx.BoolConst("learning-theory-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("learning-theory-child")
	must(err)
	closeExpr(t, childTerm)
	parent, err := source.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := source.CreateNode(childTerm)
	must(err)
	negative, err := p.Not()
	must(err)
	closeExpr(t, negative)
	body, err := negative.And(childTerm)
	must(err)
	closeExpr(t, body)
	childDecl, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDecl)
	parentRule, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, body)
	must(err)
	defer func() { must(parentRule.Close()) }()
	truth, err := ctx.BoolVal(true)
	must(err)
	closeExpr(t, truth)
	childRule, err := NewTransformer(ctx, nil, nil, truth)
	must(err)
	defer func() { must(childRule.Close()) }()
	parentSourceEdge, err := source.CreateEdge(parent, parentRule, []*Node{child})
	must(err)
	childSourceEdge, err := source.CreateEdge(child, childRule, nil)
	must(err)

	unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer func() { must(unwinding.Close()) }()
	mappedParent, err := unwinding.CloneNode(parent)
	must(err)
	mappedChild, err := unwinding.CloneNode(child)
	must(err)
	parentEdge, err := unwinding.CreateEdge(mappedParent, parentRule, []*Node{mappedChild})
	must(err)
	parentEdge.Map = parentSourceEdge
	childEdge, err := unwinding.CreateEdge(mappedChild, childRule, nil)
	must(err)
	childEdge.Map = childSourceEdge

	working, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer func() { must(working.Close()) }()
	must(working.BindSolver(main))
	base, err := NewDerivationTree(unwinding, working)
	must(err)
	base.FullExpand = true
	base.FalseApprox = false
	search, err := NewBacktrackingDerivation(base)
	must(err)
	top, err := base.CreateApproximatedInstance(mappedParent)
	must(err)
	base.Top = top
	theory := []*Expr{p}
	checker, err := NewLocalEdgeChecker(source, factory, theory)
	must(err)
	defer func() { must(checker.Close()) }()

	ph, err := p.Raw()
	must(err)
	must(main.Push())
	defer func() { must(main.Pop(1)) }()
	must(main.Assert(ph))
	must(aux.Push())
	defer func() { must(aux.Pop(1)) }()
	must(aux.Assert(ph))

	workingScopes := uint32(0)
	defer func() {
		if workingScopes != 0 {
			must(working.Pop(workingScopes))
		}
	}()
	must(working.Push())
	workingScopes++
	must(working.AssertNode(top))
	must(working.Push())
	workingScopes++
	base.RemoveLeaves(map[*Node]bool{top: true})
	must(search.ExpandMaterializedNode(top))
	instanceChild := top.Outgoing.Children[0]
	must(working.Push())
	workingScopes++
	base.RemoveLeaves(map[*Node]bool{instanceChild: true})
	must(search.ExpandMaterializedNode(instanceChild))
	status, err := working.Check(top, nil, nil)
	must(err)
	if status != lbool.False {
		t.Fatal("background did not refute expanded derivation", status)
	}
	cases := &CaseInterpolator{Theory: theory}
	_, err = search.LearnCurrentExpansions(checker, aux, manager, cases, nil)
	must(err)

	learned, err := mappedChild.Annotation.Formula()
	must(err)
	defer func() { must(learned.Close()) }()
	lh, err := learned.Raw()
	must(err)
	notLearned, err := m.MkNot(lh)
	must(err)
	oracle, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer func() { must(oracle.Close()) }()
	must(oracle.Push())
	must(oracle.Assert(ph))
	must(oracle.Assert(notLearned))
	status, err = oracle.CheckSat(nil)
	must(err)
	must(oracle.Pop(1))
	if status != lbool.False {
		t.Fatal("background does not entail learned annotation", status)
	}
	must(oracle.Push())
	must(oracle.Assert(ph))
	must(oracle.Assert(lh))
	status, err = oracle.CheckSat(nil)
	must(err)
	must(oracle.Pop(1))
	if status != lbool.True {
		t.Fatal("learned annotation is inconsistent with background", status)
	}

	must(working.Pop(workingScopes))
	workingScopes = 0
}
