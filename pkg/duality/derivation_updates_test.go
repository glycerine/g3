package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDerivationUpdatesPropagateAcrossInstancesAndBacktracking(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	aux, err := factory.New(m, nil, false, false, false, "")
	must(err)
	defer aux.Close()
	term, err := ctx.BoolConst("update-R")
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
	for _, node := range []*Node{first, second} {
		activation, vars, err := working.RedVars(node)
		must(err)
		if len(vars) != 0 {
			t.Fatal("unexpected parameters")
		}
		must(working.ConstrainEdgeLocalized(top.Outgoing, activation))
		must(activation.Close())
	}
	base.RemoveLeaves(map[*Node]bool{first: true})
	must(working.Push())
	must(search.ExpandMaterializedNode(first))
	if !search.AtCurrentStackLevel(first) || search.AtCurrentStackLevel(top) {
		t.Fatal("current expansion membership")
	}
	must(first.Annotation.SetEmpty())
	changed, err := search.RecordAnnotationUpdate(first, aux)
	must(err)
	if !changed || len(search.updatedNodes) != 2 || search.updatedNodes[0] != first || search.updatedNodes[1] != second {
		t.Fatal("older instances not queued")
	}
	empty, err := second.Annotation.IsEmpty()
	must(err)
	if !empty {
		t.Fatal("sibling did not receive learned fact")
	}
	changed, err = search.RecordAnnotationUpdate(first, aux)
	must(err)
	if changed || len(search.updatedNodes) != 2 {
		t.Fatal("redundant update was queued")
	}
	before, err := s.CheckSat(nil)
	must(err)
	if before != lbool.True {
		t.Fatal("graph already inconsistent")
	}
	must(search.HandleUpdatedNodes())
	after, err := s.CheckSat(nil)
	must(err)
	if after != lbool.False {
		t.Fatal("updated annotations not asserted")
	}
	must(search.RemoveUpdateNodesAtCurrentLevel())
	if len(search.updatedNodes) != 2 {
		t.Fatal("older updates incorrectly removed")
	}
	must(search.PopLevel())
	// The creator of both updated instances is now the current level.
	must(search.HandleUpdatedNodes())
	if len(search.updatedNodes) != 0 {
		t.Fatal("current-level updates did not retire")
	}
	must(search.PopLevel())
	if !first.closed || !second.closed {
		t.Fatal("updated instances not deleted")
	}
	fresh, err := base.CreateApproximatedInstance(child)
	must(err)
	falsity, err := ctx.BoolVal(false)
	must(err)
	defer falsity.Close()
	falseRelation, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer falseRelation.Close()
	impliesFalse, err := fresh.Annotation.SubsetEq(falseRelation, aux)
	must(err)
	if !impliesFalse {
		t.Fatal("original learned annotation lost after backtracking")
	}
	must(working.Pop(1))
}

// AUDIT.md duality_03-F1: DerivationTreeSlow::RecordUpdate goes through
// Duality::UpdateNodeToNode, so a learned fact marks the source node in the
// outer solver's updated_nodes.
func TestRecordAnnotationUpdateMarksOuterUpdatedNodes(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("record-update-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	unwinding, err := NewUnwinding(source, graph, covering)
	must(err)
	instance, err := unwinding.CreateNodeInstance(relation, 0)
	must(err)
	working, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer working.Close()
	base, err := NewDerivationTree(graph, working)
	must(err)
	search, err := NewBacktrackingDerivation(base)
	must(err)
	search.sourceCovering = covering
	search.sourceUnwinding = unwinding
	top, err := base.CreateApproximatedInstance(instance)
	must(err)
	base.Top = top
	if unwinding.updatedNodes[relation] {
		t.Fatal("source node marked before any update")
	}
	must(top.Annotation.SetEmpty())
	changed, err := search.RecordAnnotationUpdate(top, aux)
	must(err)
	if !changed {
		t.Fatal("learned false was redundant")
	}
	if !unwinding.updatedNodes[relation] {
		t.Fatal("outer updated_nodes does not hold the source node")
	}
}
