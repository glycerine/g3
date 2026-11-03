package duality

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestDualityPopLevelRefusesInvalidOwnershipBeforeSolverMutation(t *testing.T) {
	ctx, manager, native := dualityNativeSolverFixture(t)
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
	must(working.BindSolver(native))

	parentTerm, err := ctx.BoolConst("pop-preflight-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("pop-preflight-child")
	must(err)
	closeExpr(t, childTerm)
	parent, err := original.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := original.CreateNode(childTerm)
	must(err)
	_, err = original.CreateEdge(parent, parent.Annotation, []*Node{child})
	must(err)
	_, err = original.CreateLowerBoundEdge(child)
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
	expandedChild := top.Outgoing.Children[0]

	assertUnchanged := func(label string) {
		t.Helper()
		depth, err := native.ScopeLevel()
		must(err)
		mapped := search.nodeMap[child]
		found := false
		for _, candidate := range mapped {
			found = found || candidate == expandedChild
		}
		if depth != 2 || len(search.stack) != 2 || top.Outgoing == nil || expandedChild.closed || !found || len(base.leaves) != 1 || base.leaves[0] != expandedChild {
			t.Fatal(label, "mutated derivation before ownership preflight", depth, len(search.stack), found, len(base.leaves))
		}
	}

	search.updatedNodes = []*Node{expandedChild}
	if err = search.PopLevel(); !errors.Is(err, ErrUpdatedExpansionChild) {
		t.Fatal("updated expansion child was not refused", err)
	}
	assertUnchanged("updated child")
	search.updatedNodes = nil

	mapping := append([]*Node(nil), search.nodeMap[child]...)
	search.nodeMap[child] = nil
	if err = search.PopLevel(); !errors.Is(err, ErrUnmapNode) {
		t.Fatal("missing expansion mapping was not refused", err)
	}
	search.nodeMap[child] = mapping
	assertUnchanged("missing mapping")

	edge := top.Outgoing
	formula, err := manager.RefGet(edge.F.formula)
	must(err)
	stale := edge.F.formula
	must(manager.DelRef(&edge.F.formula))
	edge.F.formula = stale
	childName, err := manager.RefGet(expandedChild.Name)
	must(err)
	staleChildName := expandedChild.Name
	must(manager.DelRef(&expandedChild.Name))
	expandedChild.Name = staleChildName
	if err = search.PopLevel(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("edge cleanup refusal was not preserved", err)
	}
	depth, err := native.ScopeLevel()
	must(err)
	if depth != 1 || len(search.stack) != 2 || !search.stack[1].popped || top.Outgoing == nil || expandedChild.closed || len(search.nodeMap[child]) != 1 || len(base.leaves) != 1 || base.leaves[0] != expandedChild {
		t.Fatal("edge cleanup refusal committed logical derivation state", depth, len(search.stack), search.stack[1].popped, len(search.nodeMap[child]), len(base.leaves))
	}
	edge.F.formula, err = manager.NewRef(formula)
	must(err)
	if err = search.PopLevel(); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("node cleanup refusal was not preserved", err)
	}
	depth, err = native.ScopeLevel()
	must(err)
	if depth != 1 || len(search.stack) != 2 || !search.stack[1].popped || top.Outgoing != nil || expandedChild.closed || len(search.nodeMap[child]) != 1 || len(base.leaves) != 1 || base.leaves[0] != expandedChild {
		t.Fatal("node cleanup refusal committed logical derivation state", depth, len(search.stack), search.stack[1].popped, len(search.nodeMap[child]), len(base.leaves))
	}
	expandedChild.Name, err = manager.NewRef(childName)
	must(err)
	must(search.PopLevel())
	depth, err = native.ScopeLevel()
	must(err)
	if depth != 1 || len(search.stack) != 1 || top.Outgoing != nil || !expandedChild.closed || len(search.nodeMap[child]) != 0 || len(base.leaves) != 1 || base.leaves[0] != top {
		t.Fatal("repaired derivation did not pop exactly one level", depth, len(search.stack), len(search.nodeMap[child]), len(base.leaves))
	}
	must(working.Pop(1))
}
