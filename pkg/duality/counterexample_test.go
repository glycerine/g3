package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityCounterexampleExclusiveGraphOwnershipAndSwap(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("cex-owned-R")
	must(err)
	closeExpr(t, term)
	graph := func() (*RPFP, *Node) {
		g, err := NewRPFP(ctx)
		must(err)
		node, err := g.CreateNode(term)
		must(err)
		t.Cleanup(func() { must(g.Close()) })
		return g, node
	}
	a, ar := graph()
	b, br := graph()
	c, cr := graph()
	left, right := NewCounterexample(), NewCounterexample()
	defer left.Close()
	defer right.Close()
	must(left.Set(a, ar))
	must(right.Set(b, br))
	must(left.Swap(right))
	if left.Tree() != b || left.Root() != br || right.Tree() != a || right.Root() != ar {
		t.Fatal("counterexample swap did not move graph/root ownership")
	}
	if err := left.Set(b, br); !errors.Is(err, ErrWrapperOwner) || b.closed {
		t.Fatal("same-graph assignment destroyed owned result", err)
	}
	copied := *right
	if err := copied.Clear(); !errors.Is(err, ErrWrapperOwner) || a.closed {
		t.Fatal("copied owner released original graph", err)
	}
	must(left.Set(c, cr))
	if !b.closed || c.closed {
		t.Fatal("replacing result did not release only its old graph")
	}
	must(left.Clear())
	if !c.closed || left.Tree() != nil || left.Root() != nil {
		t.Fatal("clear did not release graph/root")
	}
	d, dr := graph()
	must(left.Set(d, dr))
	must(left.Close())
	if !d.closed {
		t.Fatal("cleared counterexample could not own a new result")
	}
	must(right.Close())
	if !a.closed {
		t.Fatal("swapped graph ownership lost")
	}
}

func TestDualityFullBoundShortcutPreservesExistingCounterexample(t *testing.T) {
	ctx, m, aux, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("full-bound-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	outer, err := NewUnwinding(source, graph, covering)
	must(err)
	node, err := outer.CreateNodeInstance(relation, 0)
	must(err)
	prior, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	root, err := prior.CreateNode(term)
	must(err)
	cex := NewCounterexample()
	defer cex.Close()
	must(cex.Set(prior, root))
	before := m.Stats()
	outer.lastDecisions = 731
	satisfied, err := outer.SatisfyUpperBound(node, nil, cex, false)
	must(err)
	after := m.Stats()
	if !satisfied || cex.Tree() != prior || prior.closed || before.LiveReferences != after.LiveReferences || outer.lastDecisions != 731 {
		t.Fatal("full-bound shortcut allocated or replaced prior result")
	}
	satisfied, err = outer.SatisfyUpperBound(node, nil, nil, true)
	must(err)
	if !satisfied {
		t.Fatal("full-bound shortcut required derivation services")
	}
	// An unresolved non-full query must close its partial working graph,
	// restore the native scope, and preserve the older counterexample owner.
	must(node.Bound.SetEmpty())
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	inference, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer inference.Close()
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	working, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(aux))
	must(working.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	baseSearch, err := NewDerivationTree(graph, working)
	must(err)
	derivation, err := NewBacktrackingDerivation(baseSearch)
	must(err)
	checker, err := NewLocalEdgeChecker(source, factory, nil)
	must(err)
	defer checker.Close()
	query := &DerivationSearch{Search: derivation, Checker: checker, Auxiliary: inference, Simplifier: manager}
	if _, err := outer.SatisfyUpperBound(node, query, cex, false); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("unresolved bound failure missing", err)
	}
	depth, err := aux.ScopeLevel()
	must(err)
	if !working.closed || cex.Tree() != prior || prior.closed || depth != 0 || outer.lastDecisions != 731 {
		t.Fatal("bound failure leaked graph/scope or replaced prior result")
	}

	// Completion has no full-bound shortcut and must reject aliased ownership
	// before consuming an existing result or the borrowed unwinding graph.
	for _, aliased := range []*RPFP{prior, graph} {
		aliasedBase, err := NewDerivationTree(graph, aliased)
		must(err)
		aliasedSearch, err := NewBacktrackingDerivation(aliasedBase)
		must(err)
		aliasedQuery := &DerivationSearch{Search: aliasedSearch}
		if err := outer.BuildFullCex(node, aliasedQuery, cex, false); !errors.Is(err, ErrWrapperOwner) {
			t.Fatal("aliased completion accepted", err)
		}
		if aliased.closed || cex.Tree() != prior {
			t.Fatal("validation consumed borrowed graph")
		}
	}
	completion, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer completion.Close()
	must(completion.BindSolver(aux))
	must(completion.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	completionBase, err := NewDerivationTree(graph, completion)
	must(err)
	completionSearch, err := NewBacktrackingDerivation(completionBase)
	must(err)
	completionQuery := &DerivationSearch{Search: completionSearch, Checker: checker, Auxiliary: inference, Simplifier: manager}
	if err := outer.BuildFullCex(node, completionQuery, cex, true); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("unresolved completion failure missing", err)
	}
	depth, err = aux.ScopeLevel()
	must(err)
	if !completion.closed || cex.Tree() != prior || prior.closed || depth != 0 {
		t.Fatal("completion failure leaked graph/scope or replaced prior result")
	}
}
