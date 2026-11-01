package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestLocalPropagationCopiesMappedFactsAndOnlyPublishesSuccessfulTrials(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	r, err := ctx.BoolConst("local-propagate-R")
	must(err)
	closeExpr(t, r)
	p, err := ctx.BoolConst("local-propagate-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("local-propagate-q")
	must(err)
	closeExpr(t, q)
	pair, err := p.And(q)
	must(err)
	closeExpr(t, pair)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(r)
	must(err)
	pd, err := p.Decl()
	must(err)
	must(source.DeclareConstant(pd))
	must(pd.Close())
	rule, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer rule.Close()
	sourceEdge, err := source.CreateEdge(relation, rule, []*Node{relation})
	must(err)
	unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer unwinding.Close()
	parent, err := unwinding.CloneNode(relation)
	must(err)
	child, err := unwinding.CloneNode(relation)
	must(err)
	childFact, err := NewTransformer(ctx, nil, nil, pair)
	must(err)
	defer childFact.Close()
	must(replaceNodeAnnotation(child, childFact))
	unwindingEdge, err := unwinding.CreateEdge(parent, rule, []*Node{child})
	must(err)
	unwindingEdge.Map = sourceEdge
	derivation, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer derivation.Close()
	instance, err := derivation.CloneNode(parent)
	must(err)
	childInstance, err := derivation.CloneNode(child)
	must(err)
	instanceEdge, err := derivation.CreateEdge(instance, rule, []*Node{childInstance})
	must(err)
	instanceEdge.Map = sourceEdge
	checker, err := NewLocalEdgeChecker(source, factory, nil)
	must(err)
	defer checker.Close()
	before, err := m.ProofsEnabled()
	must(err)
	changed, err := checker.Propagate(instance)
	must(err)
	if !changed {
		t.Fatal("local propagation failed")
	}
	fact, err := instance.Annotation.Formula()
	must(err)
	h, err := fact.Raw()
	must(err)
	want, err := p.Raw()
	must(err)
	if h != want {
		t.Fatal("wrong published propagation fact")
	}
	must(fact.Close())
	full, err := parent.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("propagation changed mapped source annotation")
	}
	// Reflect the separately learned fact into the unwinding, then the only
	// remaining candidate q fails. The instance must keep its successful p.
	must(parent.Annotation.IntersectWith(instance.Annotation))
	changed, err = checker.Propagate(instance)
	must(err)
	if changed {
		t.Fatal("failed trial reported success")
	}
	fact, err = instance.Annotation.Formula()
	must(err)
	h, err = fact.Raw()
	must(err)
	if h != want {
		t.Fatal("failed trial overwrote instance fact")
	}
	must(fact.Close())
	cloned, err := checker.clones.GetEdgeClone(sourceEdge)
	must(err)
	trial, err := cloned.Parent.Annotation.Formula()
	must(err)
	h, err = trial.Raw()
	must(err)
	qh, err := q.Raw()
	must(err)
	if h != qh {
		t.Fatal("source checker trial state not preserved")
	}
	must(trial.Close())
	after, err := m.ProofsEnabled()
	must(err)
	if after != before || checker.graph.solver != nil || len(checker.pool.states) != 1 {
		t.Fatal("local propagation scope not restored")
	}
}
