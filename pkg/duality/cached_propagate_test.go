package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestCachedPropagationChecksFactsAndPreservesSourceFailedTrialState(t *testing.T) {
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
	relation, err := ctx.BoolConst("propagate-R")
	must(err)
	closeExpr(t, relation)
	p, err := ctx.BoolConst("propagate-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("propagate-q")
	must(err)
	closeExpr(t, q)
	pair, err := p.And(q)
	must(err)
	closeExpr(t, pair)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	original, err := source.CreateNode(relation)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	parent, err := graph.CloneNode(original)
	must(err)
	child, err := graph.CloneNode(original)
	must(err)
	annotation, err := NewTransformer(ctx, nil, nil, pair)
	must(err)
	must(child.Annotation.Close())
	child.Annotation = annotation
	pd, err := p.Decl()
	must(err)
	must(graph.DeclareConstant(pd))
	must(pd.Close())
	// This rule proves p without forcing the child's activation, so q cannot
	// be inferred merely from the child's annotation.
	formula, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer formula.Close()
	edge, err := graph.CreateEdge(parent, formula, []*Node{child})
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
	changed, err := cached.PropagateCache(edge)
	must(err)
	if !changed {
		t.Fatal("entailed child fact not propagated")
	}
	got, err := parent.Annotation.Formula()
	must(err)
	h, err := got.Raw()
	must(err)
	want, err := p.Raw()
	must(err)
	if h != want {
		t.Fatal("propagation included an unproved fact")
	}
	must(got.Close())
	// The source mutates the parent while testing candidates and does not
	// restore it when all remaining candidates fail; preserve this explicitly.
	changed, err = cached.PropagateCache(edge)
	must(err)
	if changed {
		t.Fatal("unproved remaining candidate accepted")
	}
	got, err = parent.Annotation.Formula()
	must(err)
	h, err = got.Raw()
	must(err)
	want, err = q.Raw()
	must(err)
	if h != want {
		t.Fatal("source failed-trial annotation state changed")
	}
	must(got.Close())
	// No new trial is made once the only available candidate is already present.
	must(child.Annotation.SetFull())
	changed, err = cached.PropagateCache(edge)
	must(err)
	if changed {
		t.Fatal("true child generated candidates")
	}
	must(parent.Annotation.SetFull())
	both, err := NewTransformer(ctx, nil, nil, pair)
	must(err)
	defer both.Close()
	must(replaceNodeAnnotation(child, both))
	qd, err := q.Decl()
	must(err)
	must(graph.DeclareConstant(qd))
	must(qd.Close())
	bothEdge, err := graph.CreateEdge(parent, both, []*Node{child})
	must(err)
	changed, err = cached.PropagateCache(bothEdge)
	must(err)
	if !changed {
		t.Fatal("multiple entailed facts not propagated")
	}
	got, err = parent.Annotation.Formula()
	must(err)
	h, err = got.Raw()
	must(err)
	want, err = pair.Raw()
	must(err)
	if h != want {
		t.Fatal("multiple proved facts not conjoined")
	}
	must(got.Close())

}
