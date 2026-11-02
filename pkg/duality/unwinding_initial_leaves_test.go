package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"testing"
)

func TestDualityInitialLeavesNativeEarlyExpansionAndStratification(t *testing.T) {
	for _, stratified := range []bool{false, true} {
		name := "ordinary"
		if stratified {
			name = "stratified"
		}
		t.Run(name, func(t *testing.T) {
			ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
			must(err)
			defer manager.Close()
			source, err := NewRPFP(ctx)
			must(err)
			defer source.Close()
			parentTerm, err := ctx.BoolConst("initial-leaf-parent")
			must(err)
			closeExpr(t, parentTerm)
			childTerm, err := ctx.BoolConst("initial-leaf-child")
			must(err)
			closeExpr(t, childTerm)
			childDecl, err := childTerm.Decl()
			must(err)
			defer childDecl.Close()
			parent, err := source.CreateNode(parentTerm)
			must(err)
			child, err := source.CreateNode(childTerm)
			must(err)
			rule, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
			must(err)
			defer rule.Close()
			_, err = source.CreateEdge(parent, rule, []*Node{child})
			must(err)
			_, err = source.CreateLowerBoundEdge(child)
			must(err)
			graph, err := NewRPFPWithVocabulary(source.vocabulary)
			must(err)
			defer graph.Close()
			covering, err := NewCovering(graph, native)
			must(err)
			u, err := NewUnwinding(source, graph, covering)
			must(err)
			u.StratifiedInlining = stratified
			must(u.CreateEdgesByChildMap())
			// Source initialization clears these lists, without consuming graph nodes.
			stale, err := u.CreateNodeInstance(parent, 0)
			must(err)
			u.leaves = []*Node{stale}
			must(native.Push())
			caller, err := ctx.BoolConst("initial-leaf-caller")
			must(err)
			closeExpr(t, caller)
			raw, err := caller.Raw()
			must(err)
			must(native.Assert(raw))
			_, err = native.CheckSat(nil)
			must(err)
			baseline, err := native.NumAssertions()
			must(err)
			var services *ExpansionSearch
			if !stratified {
				services = &ExpansionSearch{Native: native, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Simplifier: manager}
			}
			must(u.CreateLeaves(services))
			if len(u.leaves) != 2 || u.leaves[0].Map != parent || u.leaves[1].Map != child || stale.closed {
				t.Fatal("source leaf order/reset/graph ownership")
			}
			for _, leaf := range u.leaves {
				full, err := leaf.Annotation.IsFull()
				must(err)
				if full != stratified || leaf.Outgoing == nil || leaf.Outgoing.Map != nil || len(leaf.Outgoing.Children) != 0 {
					t.Fatal("initial leaf annotation/edge")
				}
				if stratified {
					edgeFull, err := leaf.Outgoing.F.IsFull()
					must(err)
					if !edgeFull || !u.overapproximations[leaf] || u.leafMap[leaf.Map] != leaf || len(covering.instances[leaf.Map]) != 0 || u.updatedNodes[leaf] {
						t.Fatal("stratified overapproximation registration")
					}
				} else {
					empty, err := leaf.Outgoing.F.IsEmpty()
					must(err)
					if !empty || !u.updatedNodes[leaf] || len(covering.instances[leaf.Map]) != 1 || covering.instances[leaf.Map][0] != leaf {
						t.Fatal("ordinary false leaf was not activated/marked updated")
					}
				}
			}
			expectedPending := 0
			if stratified {
				expectedPending = 2
			}
			if len(u.PendingNodes()) != expectedPending {
				t.Fatal("early expansion/pending mode", len(u.PendingNodes()), expectedPending)
			}
			if _, ok := u.NextCandidate(); ok {
				t.Fatal("false initial leaves generated a feasible induction failure")
			}
			must(u.NullaryCandidates())
			candidate, ok := u.NextCandidate()
			if !ok || candidate.Edge != child.Outgoing || len(candidate.Children) != 0 {
				t.Fatal("nullary source-edge seeding missing")
			}
			if _, ok = u.NextCandidate(); ok {
				t.Fatal("nonnullary edge seeded as an initial candidate")
			}
			depth, err := native.ScopeLevel()
			must(err)
			assertions, err := native.NumAssertions()
			must(err)
			if depth != 1 || assertions != baseline {
				t.Fatal("initialization disturbed caller solver scopes/assertions")
			}
			must(native.Pop(1))
		})
	}
}

func TestDualityMakeLeafStratifiedEarlyExpansionKeepsFalseRule(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("stratified-early-leaf")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	u.StratifiedInlining = true
	must(u.CreateEdgesByChildMap())
	node, err := u.CreateNodeInstance(relation, 0)
	must(err)
	services := &ExpansionSearch{Native: native, Evaluator: derivationEvaluatorFixture(t, ctx, globals), Simplifier: manager}
	must(u.MakeLeaf(node, false, services))
	full, err := node.Annotation.IsFull()
	must(err)
	empty, err := node.Outgoing.F.IsEmpty()
	must(err)
	if !full || !empty || len(u.PendingNodes()) != 0 || len(covering.instances[relation]) != 1 || u.updatedNodes[node] || u.overapproximations[node] {
		t.Fatal("stratified MakeLeaf differs from false rule/full annotation early expansion")
	}
}
