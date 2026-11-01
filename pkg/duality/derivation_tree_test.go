package duality

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDerivationInstancesMaterializeAndRecycleSourceEdges(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	parentTerm, err := ctx.BoolConst("derive-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("derive-child")
	must(err)
	closeExpr(t, childTerm)
	childDecl, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDecl)
	original, err := NewRPFP(ctx)
	must(err)
	defer func() { must(original.Close()) }()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer func() { must(working.Close()) }()
	must(working.BindSolver(s))
	parent, err := original.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := original.CreateNode(childTerm)
	must(err)
	formula, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
	must(err)
	defer formula.Close()
	source, err := original.CreateEdge(parent, formula, []*Node{child})
	must(err)
	source.Map = source
	derivation, err := NewDerivationTree(original, working)
	must(err)
	instance, err := derivation.CreateApproximatedInstance(parent)
	must(err)
	if instance.Map != parent || len(derivation.leaves) != 1 || derivation.leaves[0] != instance {
		t.Fatal("initial instance mapping")
	}
	must(working.Push())
	must(working.AssertNode(instance))
	must(derivation.ExpandMaterializedNode(instance))
	edge := instance.Outgoing
	if edge == nil || edge.Map != source || len(edge.Children) != 1 || edge.Children[0].Map != child {
		t.Fatal("materialized edge mapping")
	}
	if len(derivation.leaves) != 2 || derivation.leaves[1] != edge.Children[0] {
		t.Fatal("new child not registered")
	}
	got, err := s.CheckSat(nil)
	must(err)
	if got != lbool.True {
		t.Fatal("expanded graph not SAT", got)
	}
	must(working.Pop(1))
	// Recycled instances pick up the source's latest learned annotation.
	must(child.Annotation.SetEmpty())
	derivation.leaves = nil
	must(working.Push())
	must(working.AssertNode(instance))
	must(derivation.ExpandMaterializedNode(instance))
	if instance.Outgoing != edge || len(working.nodes) != 2 || len(derivation.leaves) != 1 {
		t.Fatal("recycling created new graph structure")
	}
	empty, err := edge.Children[0].Annotation.IsEmpty()
	must(err)
	if !empty {
		t.Fatal("recycled annotation stale")
	}
	got, err = s.CheckSat(nil)
	must(err)
	if got != lbool.False {
		t.Fatal("updated child annotation not asserted", got)
	}
	must(working.Pop(1))
	// Full expansion deliberately omits child overapproximation constraints.
	derivation.FullExpand = true
	derivation.leaves = nil
	must(working.Push())
	must(working.AssertNode(instance))
	must(derivation.ExpandMaterializedNode(instance))
	got, err = s.CheckSat(nil)
	must(err)
	if got != lbool.True {
		t.Fatal("full expansion asserted child approximation", got)
	}
	must(working.Pop(1))
}
