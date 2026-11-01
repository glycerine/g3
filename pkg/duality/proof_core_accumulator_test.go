package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityProofCoreAccumulatesNativeProofsAcrossChecks(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	must(graph.BindSolver(native))
	core, err := NewProofCoreAccumulator(ctx)
	must(err)
	defer func() { must(core.Close()) }()
	p, err := ctx.BoolConst("case-core-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("case-core-q")
	must(err)
	closeExpr(t, q)
	np, err := p.Not()
	must(err)
	closeExpr(t, np)
	nq, err := q.Not()
	must(err)
	closeExpr(t, nq)
	raw := func(term *Expr) ast.ASTHandle { t.Helper(); h, err := term.Raw(); must(err); return h }
	if err := core.AddCurrentProof(graph); err == nil {
		t.Fatal("missing proof accepted")
	}
	assertPair := func(positive, negative *Expr) {
		t.Helper()
		must(graph.Push())
		must(native.Assert(raw(positive)))
		must(native.Assert(raw(negative)))
		status, err := graph.Check(nil, nil, nil)
		must(err)
		if status != lbool.False {
			t.Fatal("expected native contradiction", status)
		}
	}
	assertPair(p, np)
	must(core.AddCurrentProof(graph))
	initial := len(core.terms)
	must(core.AddCurrentProof(graph))
	if len(core.terms) != initial {
		t.Fatal("repeated proof duplicated leaves")
	}
	for _, term := range []*Expr{p, np} {
		found, err := core.Contains(term)
		must(err)
		if !found {
			t.Fatal("first native proof leaf missing")
		}
	}
	found, err := core.Contains(q)
	must(err)
	if found {
		t.Fatal("unasserted term in proof union")
	}
	must(graph.ComputeProofCore())
	for h, ref := range graph.proofCore {
		if core.terms[h] == ref {
			t.Fatal("independent proof sets alias owning refs")
		}
	}
	must(graph.Pop(1))
	assertPair(q, nq)
	must(core.AddCurrentProof(graph))
	for _, term := range []*Expr{p, np, q, nq} {
		found, err := core.Contains(term)
		must(err)
		if !found {
			t.Fatal("proof leaf lost across pop/check")
		}
	}
	// Replace a graph core for only the second proof with the accumulated union.
	must(graph.ComputeProofCore())
	if _, found := graph.proofCore[raw(p)]; found {
		t.Fatal("single-proof graph cache included old proof")
	}
	must(core.Install(graph))
	for _, term := range []*Expr{p, np, q, nq} {
		if _, found := graph.proofCore[raw(term)]; !found {
			t.Fatal("installed union lost a source proof leaf")
		}
	}
	if err := core.AddCurrentProof(graph); err == nil {
		t.Fatal("consumed accumulator accepted proof")
	}
	must(core.Close())
	if len(graph.proofCore) < 4 {
		t.Fatal("closing consumed accumulator released transferred set")
	}
	must(graph.Pop(1))
	status, err := graph.Check(nil, nil, nil)
	must(err)
	if status != lbool.True || graph.proofCore != nil {
		t.Fatal("next graph check did not invalidate installed core")
	}
}
