package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDerivationAdvanceSATPreservesCompletedCounterexample(t *testing.T) {
	ctx, _, s, globals := dualityNativeSolverServicesFixture(t)
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
	must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	term, err := ctx.BoolConst("advance-R")
	must(err)
	closeExpr(t, term)
	root, err := original.CreateNode(term)
	must(err)
	must(root.Bound.SetEmpty())
	leaf, err := original.CreateNode(term)
	must(err)
	// Use a real child relation in the edge formula so its activation is forced.
	relation, err := term.Decl()
	must(err)
	closeDecl(t, relation)
	f, err := NewTransformer(ctx, []*FuncDecl{relation}, nil, term)
	must(err)
	defer func() { must(f.Close()) }()
	_, err = original.CreateEdge(root, f, []*Node{leaf})
	must(err)
	_, err = original.CreateLowerBoundEdge(leaf)
	must(err)
	base, err := NewDerivationTree(original, working)
	must(err)
	base.FullExpand = true
	base.FalseApprox = false
	search, err := NewBacktrackingDerivation(base)
	must(err)
	top, err := base.CreateApproximatedInstance(root)
	must(err)
	base.Top = top
	must(working.Push())
	must(working.AssertNode(top))
	check := func() {
		t.Helper()
		status, err := working.Check(top, nil, nil)
		must(err)
		if status != lbool.True {
			t.Fatal("expected model for next step", status)
		}
	}
	if _, err := search.AdvanceSatisfiable(); err == nil {
		t.Fatal("advanced without a model")
	}
	initialDepth, err := s.ScopeLevel()
	must(err)
	if initialDepth != 1 {
		t.Fatal("missing-model failure pushed a solver scope")
	}
	check()
	choices, err := search.ExpandSomeNodes(false, 0)
	must(err)
	if !choices || len(search.stack) != 1 || len(base.leaves) != 1 {
		t.Fatal("zero limit changed source choice reporting/frontier")
	}
	priority, err := search.ExpandSomeNodes(true, 1)
	must(err)
	if priority || len(search.stack) != 1 {
		t.Fatal("neutral high-priority heuristic expanded a node")
	}
	advanced, err := search.AdvanceSatisfiable()
	must(err)
	if !advanced || len(search.stack) != 2 || top.Outgoing == nil || len(base.leaves) != 1 {
		t.Fatal("root expansion did not enter backtracking stack")
	}
	child := top.Outgoing.Children[0]
	if len(search.nodeMap[leaf]) != 1 || search.nodeMap[leaf][0] != child {
		t.Fatal("missing child mapping")
	}
	check()
	advanced, err = search.AdvanceSatisfiable()
	must(err)
	if !advanced || len(search.stack) != 3 || child.Outgoing == nil || len(base.leaves) != 0 {
		t.Fatal("leaf expansion not tracked")
	}
	if len(top.Outgoing.constraints) != 1 {
		t.Fatal("preceding expansion decisions were not fixed")
	}
	check()
	advanced, err = search.AdvanceSatisfiable()
	must(err)
	if advanced || len(search.stack) != 1 {
		t.Fatal("completed derivation did not unwind expansion levels")
	}
	depth, err := s.ScopeLevel()
	must(err)
	if depth != 1 || len(working.scopes) != 2 {
		t.Fatal("completion disturbed enclosing scope", depth, len(working.scopes))
	}
	if top.Outgoing == nil || child.Outgoing == nil || child.closed || len(working.edges) != 2 || len(working.nodes) != 2 {
		t.Fatal("completed counterexample graph was deleted")
	}
	for _, edge := range working.edges {
		if edge.dual != (ast.ASTRef{}) || len(edge.constraints) != 0 {
			t.Fatal("completed graph kept popped solver state")
		}
	}
	must(working.Pop(1))
}
