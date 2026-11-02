package duality

import (
	"github.com/glycerine/g3/pkg/smtcore"
	"testing"
)

func TestDualityEagerHistoryDeductionPublishesOnlyProvedConjuncts(t *testing.T) {
	ctx, m, aux, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	old, err := NewRPFP(ctx)
	must(err)
	defer old.Close()
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	p, err := ctx.BoolConst("eager-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("eager-q")
	must(err)
	closeExpr(t, q)
	for _, graph := range []*RPFP{old, source} {
		for _, term := range []*Expr{p, q} {
			decl, err := term.Decl()
			must(err)
			must(graph.DeclareConstant(decl))
			must(decl.Close())
		}
	}
	oldTerm, err := ctx.BoolConst("eager-R@@old")
	must(err)
	closeExpr(t, oldTerm)
	newTerm, err := ctx.BoolConst("eager-R@@new")
	must(err)
	closeExpr(t, newTerm)
	oldNode, err := old.CreateNode(oldTerm)
	must(err)
	pair, err := p.And(q)
	must(err)
	closeExpr(t, pair)
	historyFact, err := NewTransformer(ctx, nil, nil, pair)
	must(err)
	defer historyFact.Close()
	must(replaceNodeAnnotation(oldNode, historyFact))
	relation, err := source.CreateNode(newTerm)
	must(err)
	rule, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer rule.Close()
	edge, err := source.CreateEdge(relation, rule, nil)
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
	_, err = outer.CreateEdgeInstance(edge, node, nil)
	must(err)
	proposer, err := NewHistoryProposer(old, source)
	must(err)
	defer proposer.Close()
	checker, err := NewLocalEdgeChecker(source, factory, nil)
	must(err)
	defer checker.Close()
	modeBefore, err := m.ProofsEnabled()
	must(err)
	must(outer.DoEagerDeduction(node, []*HistoryProposer{proposer}, checker, aux))
	pFact, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer pFact.Close()
	qFact, err := NewTransformer(ctx, nil, nil, q)
	must(err)
	defer qFact.Close()
	forward, err := node.Annotation.SubsetEq(pFact, aux)
	must(err)
	backward, err := pFact.SubsetEq(node.Annotation, aux)
	must(err)
	rejected, err := node.Annotation.SubsetEq(qFact, aux)
	must(err)
	if !forward || !backward || rejected || !outer.updatedNodes[relation] || !covering.someUpdates {
		t.Fatal("eager deduction accepted an unproved conjunct or lost proved fact")
	}
	state := checker.pool.states[edge]
	if state == nil {
		t.Fatal("dedicated cached edge solver not created")
	}
	assertions, err := state.solver.NumAssertions()
	must(err)
	outer.updatedNodes = nil
	covering.someUpdates = false
	must(outer.DoEagerDeduction(node, []*HistoryProposer{proposer}, checker, aux))
	after, err := state.solver.NumAssertions()
	must(err)
	if after != assertions || len(outer.updatedNodes) != 0 || covering.someUpdates {
		t.Fatal("repeat deduction duplicated guards or republished redundant fact")
	}
	proposals, err := proposer.Propose(node)
	must(err)
	if len(proposals) != 1 {
		t.Fatal("history proposal lost")
	}
	formula, err := proposals[0].Formula()
	must(err)
	defer formula.Close()
	got, err := formula.Raw()
	must(err)
	want, err := pair.Raw()
	must(err)
	if got != want {
		t.Fatal("per-conjunct bound mutation changed history proposal")
	}
	must(proposals[0].Close())
	full, err := relation.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("eager update changed original rule annotation")
	}
	modeAfter, err := m.ProofsEnabled()
	must(err)
	if modeBefore != modeAfter || checker.graph.solver != nil {
		t.Fatal("eager cached checks leaked solver/proof-mode binding")
	}
	must(outer.DoEagerDeduction(node, nil, checker, aux))
}
