package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestCachedGraphAssertionsPreserveLiteralVectorSourceSemantics(t *testing.T) {
	ctx, _, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	term, err := ctx.BoolConst("cached-graph-R")
	must(err)
	closeExpr(t, term)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	graph, err := NewRPFP(ctx)
	must(err)
	parent, err := graph.CreateNode(term)
	must(err)
	must(parent.Bound.SetEmpty())
	formula, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer formula.Close()
	edge, err := graph.CreateEdge(parent, formula, nil)
	must(err)
	pool, err := NewEdgeSolverPool(ctx, factory)
	must(err)
	defer pool.Close()
	state, err := pool.SolverForEdge(edge, false, false, nil)
	must(err)
	scope, err := state.Scope()
	must(err)
	defer scope.Close()
	defer graph.Close()
	cached, err := NewCachedGraphAssertions(graph, scope)
	must(err)
	var literals []*Expr
	closeVector := func(values []*Expr) {
		for _, value := range values {
			must(value.Close())
		}
	}
	must(cached.AssertEdgeCache(edge, &literals, true))
	defer func() { closeVector(literals) }()
	if len(literals) != 1 {
		t.Fatal("edge formula literal missing")
	}
	must(cached.AssertNodeCache(parent, literals))
	if len(literals) != 1 {
		t.Fatal("by-value node call appended caller literals")
	}
	status, core, err := scope.CheckCore(literals)
	must(err)
	closeVector(core)
	if status != lbool.True {
		t.Fatal("discarded node literals were implicitly assumed")
	}
	bound, err := graph.GetUpperBound(parent)
	must(err)
	boundLits, err := scope.Assumptions.GetAssumptionLits(bound, nil)
	must(err)
	must(bound.Close())
	defer closeVector(boundLits)
	combined := append(append([]*Expr(nil), literals...), boundLits...)
	status, core, err = scope.CheckCore(combined)
	must(err)
	closeVector(core)
	if status != lbool.False {
		t.Fatal("combined cached graph formulas not UNSAT")
	}
	// Explicit localized constraints append their literals and record graph
	// ownership without asserting the unguarded constraint into the solver.
	must(cached.ConstrainEdgeLocalizedCache(edge, falsity, &literals))
	if len(edge.constraints) != 1 || len(literals) != 2 {
		t.Fatal("localized constraint bookkeeping")
	}
	status, core, err = scope.CheckCore(nil)
	must(err)
	closeVector(core)
	if status != lbool.True {
		t.Fatal("cached constraint asserted unconditionally")
	}
	status, core, err = scope.CheckCore(literals)
	must(err)
	closeVector(core)
	if status != lbool.False {
		t.Fatal("localized constraint literal not enforced")
	}
	child, err := graph.CreateNode(term)
	must(err)
	must(child.Annotation.SetEmpty())
	must(child.Bound.SetEmpty())
	parentEdge, err := graph.CreateEdge(parent, parent.Annotation, []*Node{child})
	must(err)
	var childrenLits []*Expr
	defer func() { closeVector(childrenLits) }()
	must(cached.AssertEdgeCache(parentEdge, &childrenLits, false))
	if len(childrenLits) != 0 {
		t.Fatal("trivial edge shortcut did not apply")
	}
	must(cached.AssertEdgeCache(parentEdge, &childrenLits, true))
	if len(parentEdge.constraints) != 1 || len(childrenLits) == 0 {
		t.Fatal("child annotation cache path")
	}
	childBound, err := graph.GetUpperBound(child)
	must(err)
	activeChild, err := scope.Assumptions.GetAssumptionLits(childBound, nil)
	must(err)
	must(childBound.Close())
	defer closeVector(activeChild)
	combined = append(append([]*Expr(nil), childrenLits...), activeChild...)
	status, core, err = scope.CheckCore(combined)
	must(err)
	closeVector(core)
	if status != lbool.False {
		t.Fatal("cached child annotation not enforced")
	}

}
