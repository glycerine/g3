package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestLocalCachedCheckerSwitchesEdgesAndRefreshesBounds(t *testing.T) {
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
	term, err := ctx.BoolConst("local-check-R")
	must(err)
	closeExpr(t, term)
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	a, err := source.CreateNode(term)
	must(err)
	ae, err := source.CreateLowerBoundEdge(a)
	must(err)
	b, err := source.CreateNode(term)
	must(err)
	falseF, err := NewTransformer(ctx, nil, nil, falsity)
	must(err)
	defer falseF.Close()
	be, err := source.CreateEdge(b, falseF, nil)
	must(err)
	unwinding, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer unwinding.Close()
	ac, err := unwinding.CloneNode(a)
	must(err)
	aedge, err := unwinding.CreateEdge(ac, ae.F, nil)
	must(err)
	aedge.Map = ae
	bc, err := unwinding.CloneNode(b)
	must(err)
	bedge, err := unwinding.CreateEdge(bc, be.F, nil)
	must(err)
	bedge.Map = be
	bound, err := a.Bound.Clone()
	must(err)
	defer bound.Close()
	must(bound.SetEmpty())
	childTerm, err := ctx.BoolConst("local-check-child")
	must(err)
	closeExpr(t, childTerm)
	childDecl, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDecl)
	parentSource, err := source.CreateNode(term)
	must(err)
	childSource, err := source.CreateNode(childTerm)
	must(err)
	childFormula, err := NewTransformer(ctx, []*FuncDecl{childDecl}, nil, childTerm)
	must(err)
	defer childFormula.Close()
	sourceChildEdge, err := source.CreateEdge(parentSource, childFormula, []*Node{childSource})
	must(err)
	parentInstance, err := unwinding.CloneNode(parentSource)
	must(err)
	childInstance, err := unwinding.CloneNode(childSource)
	must(err)
	childEdge, err := unwinding.CreateEdge(parentInstance, childFormula, []*Node{childInstance})
	must(err)
	childEdge.Map = sourceChildEdge
	checker, err := NewLocalEdgeChecker(source, factory, nil)
	must(err)
	defer checker.Close()
	beforeMode, err := m.ProofsEnabled()
	must(err)
	check := func(edge *Edge, want lbool.LBool) {
		t.Helper()
		status, err := checker.CheckEdgeCaching(edge, bound)
		must(err)
		if status != want {
			t.Fatal("local cached check", status, want)
		}
		mode, err := m.ProofsEnabled()
		must(err)
		if mode != beforeMode || checker.graph.solver != nil || len(checker.graph.scopes) != 1 {
			t.Fatal("checker scope did not restore")
		}
	}
	check(aedge, lbool.True)
	check(bedge, lbool.False)
	must(bound.SetFull())
	check(aedge, lbool.False)
	must(bound.SetEmpty())
	check(aedge, lbool.True)
	check(childEdge, lbool.True)
	must(childInstance.Annotation.SetEmpty())
	check(childEdge, lbool.False)
	must(childInstance.Annotation.SetFull())
	check(childEdge, lbool.True)
	if len(checker.pool.states) != 3 {
		t.Fatal("edge solvers were not cached separately")
	}
	first := checker.pool.states[ae]
	initial, err := first.solver.NumAssertions()
	must(err)
	check(bedge, lbool.False)
	check(aedge, lbool.True)
	after, err := first.solver.NumAssertions()
	must(err)
	if initial != after {
		t.Fatal("repeated local checks duplicated guard assertions")
	}
}
