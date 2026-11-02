package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDerivationLearnsAndPublishesFactsAcrossBacktracking(t *testing.T) {
	ctx, m, main, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	aux, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer aux.Close()
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	source, err := NewRPFP(ctx)
	must(err)
	defer func() { must(source.Close()) }()
	p, err := ctx.BoolConst("learning-p")
	must(err)
	closeExpr(t, p)
	pd, err := p.Decl()
	must(err)
	must(source.DeclareConstant(pd))
	must(pd.Close())
	parentTerm, err := ctx.BoolConst("learning-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("learning-child")
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
	defer parentRule.Close()
	childRule, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer childRule.Close()
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
	checker, err := NewLocalEdgeChecker(source, factory, nil)
	must(err)
	defer checker.Close()
	must(working.Push())
	must(working.AssertNode(top))
	must(working.Push())
	base.RemoveLeaves(map[*Node]bool{top: true})
	must(search.ExpandMaterializedNode(top))
	instanceChild := top.Outgoing.Children[0]
	must(working.Push())
	base.RemoveLeaves(map[*Node]bool{instanceChild: true})
	must(search.ExpandMaterializedNode(instanceChild))
	checkUnsat := func() {
		t.Helper()
		status, err := working.Check(top, nil, nil)
		must(err)
		if status != lbool.False {
			t.Fatal("expected refutation", status)
		}
	}
	checkUnsat()
	updates, err := search.LearnCurrentExpansions(checker, aux, manager, nil, nil)
	must(err)
	if updates != 1 || len(search.updatedNodes) != 1 {
		t.Fatal("child fact not published/queued", updates, len(search.updatedNodes))
	}
	forward, err := mappedChild.Annotation.SubsetEq(childRule, aux)
	must(err)
	backward, err := childRule.SubsetEq(mappedChild.Annotation, aux)
	must(err)
	if !forward || !backward {
		t.Fatal("learned child annotation differs from p")
	}
	updates, err = search.LearnCurrentExpansions(checker, aux, manager, nil, nil)
	must(err)
	if updates != 0 || search.Heuristic.updates[mappedChild] != 1 || len(search.updatedNodes) != 1 {
		t.Fatal("redundant learning not penalized without duplicate queue entries")
	}
	done, err := search.BacktrackUnsatisfiable(checker, aux)
	must(err)
	if done || len(search.stack) != 2 || len(search.updatedNodes) != 0 || instanceChild.Outgoing != nil {
		t.Fatal("first backtrack did not apply learned child fact at parent level")
	}
	checkUnsat()
	updates, err = search.LearnCurrentExpansions(checker, aux, manager, nil, nil)
	must(err)
	if updates != 1 {
		t.Fatal("parent refutation not learned")
	}
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	falseFact, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer falseFact.Close()
	empty, err := mappedParent.Annotation.SubsetEq(falseFact, aux)
	must(err)
	if !empty {
		t.Fatal("parent annotation not semantically refuted")
	}
	done, err = search.BacktrackUnsatisfiable(checker, aux)
	must(err)
	if !done || len(search.stack) != 1 || top.Outgoing != nil || !instanceChild.closed {
		t.Fatal("final refutation did not unwind derivation")
	}
	sourceFull, err := parent.Annotation.IsFull()
	must(err)
	if !sourceFull {
		t.Fatal("unwinding updates changed original rule graph")
	}
	must(working.Pop(1))
}
