package smtcore

import (
	"reflect"
	"testing"
)

func TestSMTDifferenceGraphShortcutsPreserveStrictConflict(t *testing.T) {
	for _, strict := range []bool{false, true} {
		g := NewSMTInfRationalDiffGraph(testQ())
		for i := int32(0); i < 5; i++ {
			astTestMust(t, g.InitVariable(i))
		}
		if _, err := g.NegativeCycleExplanations(); err == nil {
			t.Fatal("feasible graph accepted")
		}
		add := func(source, target int32, weight *smtInfIntRational, enable bool) int32 {
			id, err := g.AddEdge(source, target, weight, NewSMTLiteral(int32(len(g.edges)+2), false))
			astTestMust(t, err)
			if enable {
				ok, err := g.EnableEdge(id)
				astTestMust(t, err)
				if !ok {
					t.Fatal("inconsistent prefix")
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
		add(0, 3, diffInf(0, 0), false)
		closing := add(4, 0, closingWeight, false)
		ok, err := g.EnableEdge(closing)
		astTestMust(t, err)
		if ok {
			t.Fatal("negative cycle accepted")
		}
		original, err := g.NegativeCycleEdges()
		astTestMust(t, err)
		if len(original) != 5 {
			t.Fatal("missing original five-edge cycle", original)
		}
		got, err := g.NegativeCycleExplanations()
		astTestMust(t, err)
		ids := []int32{closing, chain[3], full}
		if strict {
			ids = []int32{closing, chain[3], near, chain[0]}
		}
		want := []SMTLiteral{}
		for _, id := range ids {
			want = append(want, g.edges[id].explanation)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("shortened conflict/order", strict, got, want)
		}
		// Traversal borrows edge weights; modifying its local potentials must not
		// mutate graph-owned numerals or the original parent witness.
		again, err := g.NegativeCycleEdges()
		astTestMust(t, err)
		if !reflect.DeepEqual(original, again) || g.edges[full].weight.Cmp(fullWeight) != 0 {
			t.Fatal("traversal changed graph")
		}
		repeated, err := g.NegativeCycleExplanations()
		astTestMust(t, err)
		if !reflect.DeepEqual(got, repeated) {
			t.Fatal("unstable repeated explanation")
		}
	}
}
