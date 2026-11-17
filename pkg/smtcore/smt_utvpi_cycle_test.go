package smtcore

import (
	"reflect"
	"testing"
)

func TestSMTUTVPINegativeCycleShortcutsAndStrictness(t *testing.T) {
	for _, strict := range []bool{false, true} {
		g := testUTVPIGraph()
		for i := 0; i < 3; i++ {
			_, err := g.addVariable(false)
			astTestMust(t, err)
		}
		if _, err := g.negativeCycleEdges(); err == nil {
			t.Fatal("feasible graph accepted as a negative-cycle witness")
		}
		add := func(source, target int32, weight *smtInfIntRational, enable bool) int32 {
			id, err := g.graph.AddEdge(source, target, weight, NewSMTLiteral(int32(len(g.graph.edges)+2), false))
			astTestMust(t, err)
			if enable {
				ok, err := g.graph.EnableEdge(id)
				astTestMust(t, err)
				if !ok {
					t.Fatal("feasible prefix refused")
				}
			}
			return id
		}
		chain := []int32{add(0, 1, diffInf(0, 0), true), add(1, 2, diffInf(0, 0), true), add(2, 3, diffInf(0, 0), true), add(3, 4, diffInf(0, 0), true)}
		nearWeight, fullWeight, closingWeight := diffInf(1, 0), diffInf(2, 0), diffInf(-3, 0)
		if strict {
			nearWeight, fullWeight, closingWeight = diffInf(0, 1), diffInf(0, 2), diffInf(0, -2)
		}
		near := add(1, 3, nearWeight, true)
		full := add(0, 3, fullWeight, true)
		// A disabled zero-weight shortcut would make a shorter witness, but
		// cannot supply an antecedent in an actual arithmetic conflict.
		add(0, 3, diffInf(0, 0), false)
		closing := add(4, 0, closingWeight, false)
		ok, err := g.graph.EnableEdge(closing)
		astTestMust(t, err)
		if ok {
			t.Fatal("negative cycle accepted")
		}
		original, err := g.graph.NegativeCycleEdges()
		astTestMust(t, err)
		if len(original) != 5 {
			t.Fatal("test prefix did not create its five-edge parent cycle", strict, original, g.graph.parent)
		}
		shortened, err := g.negativeCycleEdges()
		astTestMust(t, err)
		want := []int32{closing, chain[3], full}
		if strict {
			// The full shortcut would raise -2*epsilon to zero. The source
			// must reject it while retaining the valid epsilon shortcut.
			want = []int32{closing, chain[3], near, chain[0]}
		}
		if !reflect.DeepEqual(shortened, want) {
			t.Fatal("source shortcut selection/order", shortened, "want", want)
		}
		var weight smtInfIntRational
		for _, id := range shortened {
			weight.Add(&weight, &g.graph.edges[id].weight)
		}
		if weight.Sign() >= 0 {
			t.Fatal("shortened explanation lost strict inconsistency")
		}
		// Traversal must preserve the failed graph and parent chain so the
		// conflict can be inspected again by proof/antecedent consumers.
		again, err := g.graph.NegativeCycleEdges()
		astTestMust(t, err)
		if !reflect.DeepEqual(original, again) {
			t.Fatal("shortcut traversal mutated the parent cycle")
		}
	}
}
