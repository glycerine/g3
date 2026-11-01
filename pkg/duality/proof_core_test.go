package duality

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestRPFPProofCoreNativeConstraints(t *testing.T) {
	ctx, _, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	term, err := ctx.BoolConst("proof-core-R")
	must(err)
	closeExpr(t, term)
	node, err := g.CreateNode(term)
	must(err)
	edge, err := g.CreateLowerBoundEdge(node)
	must(err)
	other, err := g.CreateNode(term)
	must(err)
	unused, err := g.CreateLowerBoundEdge(other)
	must(err)
	if err := g.ComputeProofCore(); err == nil {
		t.Fatal("accepted missing proof")
	}
	must(g.Push())
	falsity, err := ctx.BoolVal(false)
	must(err)
	closeExpr(t, falsity)
	must(g.ConstrainEdgeLocalized(edge, falsity))
	status, err := g.Check(node, nil, nil)
	must(err)
	if status != lbool.False {
		t.Fatal(status)
	}
	used, err := g.EdgeUsedInProof(edge)
	must(err)
	if !used {
		t.Fatal("contradictory localized constraint absent from proof core")
	}
	used, err = g.EdgeUsedInProof(unused)
	must(err)
	if used {
		t.Fatal("unasserted edge used in proof")
	}
	if len(g.proofCore) == 0 {
		t.Fatal("empty asserted proof leaves")
	}
	must(g.ComputeProofCore())
	must(g.Pop(1))
	status, err = g.Check(node, nil, nil)
	must(err)
	if status != lbool.True || g.proofCore != nil {
		t.Fatal("new check failed to invalidate proof core", status)
	}
	if err := g.ComputeProofCore(); err == nil {
		t.Fatal("accepted SAT result as proof")
	}
	must(edge.F.SetEmpty())
	must(node.Bound.SetEmpty())
	must(g.Push())
	must(g.AssertNode(node))
	must(g.AssertEdge(edge, 0, false, false))
	status, err = g.Check(node, nil, nil)
	must(err)
	if status != lbool.False {
		t.Fatal("edge formula did not refute bound", status)
	}
	used, err = g.EdgeUsedInProof(edge)
	must(err)
	if !used {
		t.Fatal("asserted edge formula absent from proof core")
	}
	if len(edge.constraints) != 0 {
		t.Fatal("constraint survived scope pop")
	}
	must(g.ClearProofCore())
	used, err = g.EdgeUsedInProof(edge)
	must(err)
	if !used {
		t.Fatal("could not recompute core from current proof")
	}
	must(g.Pop(1))
}
