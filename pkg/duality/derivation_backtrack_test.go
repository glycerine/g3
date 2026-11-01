package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDerivationBacktrackingRestoresNestedFrontiers(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("backtrack-R")
	must(err)
	closeExpr(t, term)
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(s))
	parent, err := original.CreateNode(term)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := original.CreateNode(term)
	must(err)
	leaf, err := original.CreateNode(term)
	must(err)
	_, err = original.CreateEdge(parent, parent.Annotation, []*Node{child, child})
	must(err)
	_, err = original.CreateEdge(child, child.Annotation, []*Node{leaf})
	must(err)
	_, err = original.CreateLowerBoundEdge(leaf)
	must(err)
	base, err := NewDerivationTree(original, working)
	must(err)
	search, err := NewBacktrackingDerivation(base)
	must(err)
	top, err := base.CreateApproximatedInstance(parent)
	must(err)
	base.Top = top
	must(working.Push())
	must(working.AssertNode(top))
	base.RemoveLeaves(map[*Node]bool{top: true})
	must(working.Push())
	must(search.ExpandMaterializedNode(top))
	first, second := top.Outgoing.Children[0], top.Outgoing.Children[1]
	if len(search.nodeMap[child]) != 2 || len(base.leaves) != 2 {
		t.Fatal("first expansion maps")
	}
	base.RemoveLeaves(map[*Node]bool{first: true})
	must(working.Push())
	must(search.ExpandMaterializedNode(first))
	descendant := first.Outgoing.Children[0]
	if len(working.nodes) != 4 || len(working.edges) != 2 || len(search.stack) != 3 {
		t.Fatal("nested expansion structure")
	}
	must(search.PopLevel())
	if !descendant.closed || first.Outgoing != nil || len(working.nodes) != 3 || len(working.edges) != 1 || len(base.leaves) != 2 || base.leaves[0] != second || base.leaves[1] != first {
		t.Fatal("nested pop frontier")
	}
	if len(search.nodeMap[leaf]) != 0 || len(search.nodeMap[child]) != 2 {
		t.Fatal("nested pop mapping")
	}
	status, err := s.CheckSat(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("nested pop solver state", status)
	}
	must(search.PopLevel())
	if !first.closed || !second.closed || top.Outgoing != nil || len(working.nodes) != 1 || len(working.edges) != 0 || len(base.leaves) != 1 || base.leaves[0] != top || len(search.stack) != 1 {
		t.Fatal("root expansion pop")
	}
	if len(search.nodeMap[child]) != 0 {
		t.Fatal("shared original instance mapping not drained")
	}
	if err = search.UnmapNode(top); !errors.Is(err, ErrUnmapNode) {
		t.Fatal("missing mapping error", err)
	}
	if err = search.PopLevel(); err == nil {
		t.Fatal("base backtracking level popped")
	}
	must(working.Pop(1))
}
