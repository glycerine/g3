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
