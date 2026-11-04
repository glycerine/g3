package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestCachedGeneralizationRebuildsAnnotationFromReducedCore(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	profile := NewProfiler(nil)
	ctx.activeProfiler = profile
	defer func() {
		if profile.current != profile.top || profile.top.children["Generalize"] == nil {
			t.Fatal("native phase timer scope", "Generalize")
		}
	}()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	r, err := ctx.BoolConst("generalize-R")
	must(err)
	closeExpr(t, r)
	p, err := ctx.BoolConst("generalize-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("generalize-q")
	must(err)
	closeExpr(t, q)
	disjunction, err := p.Or(q)
	must(err)
	closeExpr(t, disjunction)
	graph, err := NewRPFP(ctx)
	must(err)
	node, err := graph.CreateNode(r)
	must(err)
	annotation, err := NewTransformer(ctx, nil, nil, disjunction)
	must(err)
	must(node.Annotation.Close())
	node.Annotation = annotation
	pd, err := p.Decl()
	must(err)
	must(graph.DeclareConstant(pd))
	must(pd.Close())
	rule, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer rule.Close()
	edge, err := graph.CreateEdge(node, rule, nil)
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
	defer cached.Close()
	m.Budget().FailAfter(0)
	failure := cached.GeneralizeCache(edge, nil)
	m.Budget().FailAfter(-1)
	if failure == nil {
		t.Fatal("generalization ignored the injected allocation failure")
	}
	if profile.current != profile.top {
		t.Fatal("failed generalization left a profile scope active")
	}
	must(cached.GeneralizeCache(edge, nil))
	baseline := m.Stats().LiveReferences
	must(cached.GeneralizeCache(edge, nil))
	if got := m.Stats().LiveReferences; got != baseline {
		t.Fatal("repeated generalization leaked temporary references", got, baseline)
	}
	formula, err := node.Annotation.Formula()
	must(err)
	h, err := formula.Raw()
	must(err)
	want, err := p.Raw()
	must(err)
	if h != want {
		t.Fatal("reduced core did not reconstruct p")
	}
	must(formula.Close())
	must(cached.GeneralizeCache(edge, nil))
	formula, err = node.Annotation.Formula()
	must(err)
	h, err = formula.Raw()
	must(err)
	if h != want {
		t.Fatal("repeated generalization changed proved annotation")
	}
	must(formula.Close())
	must(node.Annotation.SetFull())
	must(cached.GeneralizeCache(edge, nil))
	full, err := node.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("true annotation shortcut changed fact")
	}
}
