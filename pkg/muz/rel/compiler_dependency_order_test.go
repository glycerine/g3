package rel

import "testing"

func finiteOrderKey(slot uint32) finitePredicateOrderKey {
	return finitePredicateOrderKey{origin: 1, slot: slot, generation: 1}
}

func TestSourceDependencyOrderAcyclicChain(t *testing.T) {
	a, b, c, d := finiteOrderKey(1), finiteOrderKey(2), finiteOrderKey(3), finiteOrderKey(4)
	order, removed := sourceDependencyOrder([]finitePredicateOrderKey{a, b, c, d}, map[finitePredicateOrderKey][]finitePredicateOrderKey{
		a: {b},
		b: {c},
		d: {a},
	})
	want := []finitePredicateOrderKey{c, b, a, d}
	if len(removed) != 0 || len(order) != len(want) {
		t.Fatalf("order=%v removed=%v", order, removed)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order=%v, want %v", order, want)
		}
	}
}

func TestSourceDependencyOrderRestrictsExternalDependencies(t *testing.T) {
	a, b, external := finiteOrderKey(1), finiteOrderKey(2), finiteOrderKey(3)
	order, removed := sourceDependencyOrder([]finitePredicateOrderKey{a, b}, map[finitePredicateOrderKey][]finitePredicateOrderKey{
		a: {b, external},
	})
	want := []finitePredicateOrderKey{b, a}
	if len(removed) != 0 || len(order) != len(want) {
		t.Fatalf("order=%v removed=%v", order, removed)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order=%v, want %v", order, want)
		}
	}
}

func TestSourceDependencyOrderBreaksAndAppendsCycleVertex(t *testing.T) {
	a, b, c, d := finiteOrderKey(1), finiteOrderKey(2), finiteOrderKey(3), finiteOrderKey(4)
	order, removed := sourceDependencyOrder([]finitePredicateOrderKey{a, b, c, d}, map[finitePredicateOrderKey][]finitePredicateOrderKey{
		a: {b},
		b: {c},
		c: {a},
		d: {a},
	})
	want := []finitePredicateOrderKey{b, a, d, c}
	if len(removed) != 1 || removed[0] != c || len(order) != len(want) {
		t.Fatalf("order=%v removed=%v", order, removed)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order=%v, want %v", order, want)
		}
	}
}

func TestSourceDependencyOrderBreaksIndependentSelfAndMultiNodeCycles(t *testing.T) {
	a, b, c, d := finiteOrderKey(1), finiteOrderKey(2), finiteOrderKey(3), finiteOrderKey(4)
	order, removed := sourceDependencyOrder([]finitePredicateOrderKey{a, b, c, d}, map[finitePredicateOrderKey][]finitePredicateOrderKey{
		a: {a},
		b: {c},
		c: {b},
		d: {a, b},
	})
	if len(removed) != 2 || removed[0] != a || removed[1] != c {
		t.Fatalf("removed=%v, want self-cycle a and DFS back-edge c", removed)
	}
	want := []finitePredicateOrderKey{b, d, a, c}
	if len(order) != len(want) {
		t.Fatalf("order=%v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order=%v, want %v", order, want)
		}
	}
}

func TestSourceDependencyOrderExhaustiveFourNodeGraphInvariants(t *testing.T) {
	nodes := []finitePredicateOrderKey{finiteOrderKey(1), finiteOrderKey(2), finiteOrderKey(3), finiteOrderKey(4)}
	for mask := 0; mask < 1<<(len(nodes)*len(nodes)); mask++ {
		dependencies := make(map[finitePredicateOrderKey][]finitePredicateOrderKey, len(nodes))
		for head := range nodes {
			for dependency := range nodes {
				bit := head*len(nodes) + dependency
				if mask&(1<<bit) != 0 {
					dependencies[nodes[head]] = append(dependencies[nodes[head]], nodes[dependency])
				}
			}
		}
		order, removed := sourceDependencyOrder(nodes, dependencies)
		if len(order) != len(nodes) {
			t.Fatalf("mask=%016b: order has %d nodes, want %d", mask, len(order), len(nodes))
		}
		position := make(map[finitePredicateOrderKey]int, len(nodes))
		for i, node := range order {
			if _, duplicate := position[node]; duplicate {
				t.Fatalf("mask=%016b: duplicate node %v in order %v", mask, node, order)
			}
			position[node] = i
		}
		removedSet := make(map[finitePredicateOrderKey]bool, len(removed))
		for i, node := range removed {
			removedSet[node] = true
			if order[len(order)-len(removed)+i] != node {
				t.Fatalf("mask=%016b: removed nodes are not the order suffix: order=%v removed=%v", mask, order, removed)
			}
		}
		for head, deps := range dependencies {
			for _, dependency := range deps {
				if !removedSet[head] && !removedSet[dependency] && position[dependency] >= position[head] {
					t.Fatalf("mask=%016b: active dependency %v does not precede %v in %v (removed %v)", mask, dependency, head, order, removed)
				}
			}
		}
	}
}
