package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestLocalGeneralizationUsesInstanceAndMappedChildAnnotations(t *testing.T) {
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
	r, err := ctx.BoolConst("local-generalize-R")
	must(err)
	closeExpr(t, r)
	rd, err := r.Decl()
	must(err)
	closeDecl(t, rd)
	p, err := ctx.BoolConst("local-generalize-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("local-generalize-q")
	must(err)
	closeExpr(t, q)
	formula, err := p.And(r)
	must(err)
	closeExpr(t, formula)
	annotation, err := p.Or(q)
	must(err)
	closeExpr(t, annotation)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(r)
	must(err)
	pd, err := p.Decl()
	must(err)
	must(source.DeclareConstant(pd))
	must(pd.Close())
	rule, err := NewTransformer(ctx, []*FuncDecl{rd}, nil, formula)
	must(err)
	defer rule.Close()
	sourceEdge, err := source.CreateEdge(relation, rule, []*Node{relation})
	must(err)
	unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer unwinding.Close()
	parent, err := unwinding.CloneNode(relation)
	must(err)
	must(parent.Annotation.SetEmpty())
	child, err := unwinding.CloneNode(relation)
	must(err)
	edge, err := unwinding.CreateEdge(parent, rule, []*Node{child})
	must(err)
	edge.Map = sourceEdge
	tree, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer tree.Close()
	instance, err := tree.CloneNode(parent)
	must(err)
	directChild, err := tree.CloneNode(child)
	must(err)
	must(directChild.Annotation.SetEmpty())
	instanceEdge, err := tree.CreateEdge(instance, rule, []*Node{directChild})
	must(err)
	instanceEdge.Map = sourceEdge
	fact, err := NewTransformer(ctx, nil, nil, annotation)
	must(err)
	defer fact.Close()
	must(replaceNodeAnnotation(instance, fact))
	checker, err := NewLocalEdgeChecker(source, factory, nil)
	must(err)
	defer checker.Close()
	before, err := m.ProofsEnabled()
	must(err)
	must(checker.Generalize(instance))
	result, err := instance.Annotation.Formula()
	must(err)
	h, err := result.Raw()
	must(err)
	want, err := p.Raw()
	must(err)
	if h != want {
		t.Fatal("wrong generalized instance annotation")
	}
	must(result.Close())
	empty, err := parent.Annotation.IsEmpty()
	must(err)
	if !empty {
		t.Fatal("generalization changed mapped parent")
	}
	full, err := child.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("generalization changed mapped child")
	}
	must(checker.Generalize(instance))
	after, err := m.ProofsEnabled()
	must(err)
	if after != before || checker.graph.solver != nil {
		t.Fatal("generalization did not restore solver scope")
	}
	instanceEdge.Map = nil
	must(checker.Generalize(instance)) // Source approximation-edge shortcut.
	result, err = instance.Annotation.Formula()
	must(err)
	h, err = result.Raw()
	must(err)
	if h != want {
		t.Fatal("unmapped-edge shortcut changed annotation")
	}
	must(result.Close())
}
