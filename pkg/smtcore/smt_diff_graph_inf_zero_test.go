package smtcore

import (
	"reflect"
	"testing"
)

func TestSMTInfRationalTightEdgeComponentsPathsAndUpdates(t *testing.T) {
	g := NewSMTInfRationalDiffGraph(testQ())
	for v := int32(0); v < 5; v++ {
		astTestMust(t, g.InitVariable(v))
	}
	edge := func(source, target int32, weight *smtInfIntRational) int32 {
		id, err := g.AddEdge(source, target, weight, NewSMTLiteral(int32(len(g.edges)+2), false))
		astTestMust(t, err)
		ok, err := g.EnableEdge(id)
		astTestMust(t, err)
		if !ok {
			t.Fatal("consistent edge rejected")
		}
		return id
	}
	edge(0, 1, diffInf(1, 1))
	edge(1, 0, diffInf(-1, -1))
	edge(1, 2, diffInf(2, 0))
	edge(2, 1, diffInf(-2, 0))
	last := edge(2, 3, diffInf(0, 0))
	direct := edge(0, 2, diffInf(3, 1))
	components, err := g.ComputeZeroEdgeSCC()
	astTestMust(t, err)
	if components[0] < 0 || components[0] != components[1] || components[0] != components[2] || components[3] != -1 || components[4] != -1 {
		t.Fatal("infinitesimal tight SCCs", components)
	}
	successors, err := g.ComputeZeroSuccessors(0)
	astTestMust(t, err)
	if !reflect.DeepEqual(successors, []int32{0, 1, 2, 3}) {
		t.Fatal("tight successors", successors)
	}
	path, found, err := g.FindShortestZeroEdgePath(0, 3)
	astTestMust(t, err)
	if !found || !reflect.DeepEqual(path, []int32{last, direct}) {
		t.Fatal("shortest tight path did not preserve source reverse callback order", path)
	}
	for _, v := range successors {
		astTestMust(t, g.IncrementAssignment(v, diffInf(-1, 0)))
	}
	if !g.IsFeasible() {
		t.Fatal("tight-successor parity adjustment broke feasibility")
	}
	astTestMust(t, g.Push())
	edge(3, 2, diffInf(0, 0))
	joined, err := g.ComputeZeroEdgeSCC()
	astTestMust(t, err)
	if joined[3] != joined[0] {
		t.Fatal("new tight reverse edge did not join the SCC")
	}
	astTestMust(t, g.Pop(1))
	restored, err := g.ComputeZeroEdgeSCC()
	astTestMust(t, err)
	if !reflect.DeepEqual(restored, components) {
		t.Fatal("popped SCC edge remained visible", restored)
	}
	if _, found, err := g.FindShortestZeroEdgePath(0, 4); err != nil || found {
		t.Fatal("disconnected vertex acquired a tight path", err)
	}
}

func TestSMTInfRationalPairedZeroAndCycleEdgeIdentity(t *testing.T) {
	g := NewSMTInfRationalDiffGraph(testQ())
	astTestMust(t, g.InitVariable(0))
	astTestMust(t, g.InitVariable(1))
	astTestMust(t, g.SetAssignment(0, diffInf(5, 2)))
	astTestMust(t, g.SetAssignment(1, diffInf(-3, -1)))
	astTestMust(t, g.SetPairToZero(0, 1))
	first, err := g.testAssignment(0)
	astTestMust(t, err)
	second, err := g.testAssignment(1)
	astTestMust(t, err)
	// The signed-pair model is (first-second)/2; source normalization
	// can join disconnected components at a common nonzero potential.
	if first.Cmp(second) != 0 || !g.IsFeasible() || len(g.edges) != 2 {
		t.Fatal("paired zero normalization did not join gauge components")
	}
	bad, err := g.AddEdge(0, 1, diffInf(0, -1), NewSMTLiteral(7, false))
	astTestMust(t, err)
	ok, err := g.EnableEdge(bad)
	astTestMust(t, err)
	if ok {
		t.Fatal("strict negative cycle accepted")
	}
	ids, err := g.NegativeCycleEdges()
	astTestMust(t, err)
	literals, err := g.NegativeCycleExplanations()
	astTestMust(t, err)
	if len(ids) != len(literals) || len(ids) != 2 || ids[0] != bad {
		t.Fatal("cycle identity traversal", ids)
	}
	for i, id := range ids {
		if g.edges[id].explanation != literals[i] {
			t.Fatal("cycle edge identity no longer matches its premise")
		}
	}
}
