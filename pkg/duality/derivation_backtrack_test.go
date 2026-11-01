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

func TestDerivationProofCoreTracksExpansionLevels(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	original, err := NewRPFP(ctx)
	must(err)
	defer func() { must(original.Close()) }()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer func() { must(working.Close()) }()
	must(working.BindSolver(s))
	base, err := NewDerivationTree(original, working)
	must(err)
	base.FullExpand = true
	search, err := NewBacktrackingDerivation(base)
	must(err)
	term, err := ctx.BoolConst("level-proof-R")
	must(err)
	closeExpr(t, term)
	unrelated, err := original.CreateNode(term)
	must(err)
	unrelatedEdge, err := original.CreateLowerBoundEdge(unrelated)
	must(err)
	must(unrelatedEdge.F.SetEmpty())
	impossible, err := original.CreateNode(term)
	must(err)
	must(impossible.Bound.SetEmpty())
	edge, err := original.CreateLowerBoundEdge(impossible)
	must(err)
	must(edge.F.SetEmpty())
	first, err := base.CreateApproximatedInstance(unrelated)
	must(err)
	last, err := base.CreateApproximatedInstance(impossible)
	must(err)
	must(working.Push())
	must(working.AssertNode(last))
	for _, node := range []*Node{first, last} {
		base.RemoveLeaves(map[*Node]bool{node: true})
		must(working.Push())
		must(search.ExpandMaterializedNode(node))
	}
	status, err := working.Check(last, nil, nil)
	must(err)
	if status != lbool.False {
		t.Fatal(status)
	}
	for level, want := range []bool{false, false, true} {
		got, err := search.LevelUsedInProof(level)
		must(err)
		if got != want {
			t.Fatalf("level %d used=%v want=%v", level, got, want)
		}
	}
	for _, level := range []int{-1, 3} {
		if _, err := search.LevelUsedInProof(level); err == nil {
			t.Fatal("accepted invalid level", level)
		}
	}
	must(search.PopLevel())
	// Source backtracking keeps the computed core after popping its solver scope.
	used, err := search.LevelUsedInProof(1)
	must(err)
	if used {
		t.Fatal("unrelated level became relevant after pop")
	}
	must(search.PopLevel())
	status, err = working.Check(last, nil, nil)
	must(err)
	if status != lbool.True || working.proofCore != nil {
		t.Fatal("post-backtrack result/core", status)
	}
	must(working.Pop(1))
}
