package interval

import "testing"

func TestModularIntervalPredicatesAndImplication(t *testing.T) {
	full := modularTestInterval(t, "0", "255", 8, true)
	wrapped := modularTestInterval(t, "240", "15", 8, true)
	inner := modularTestInterval(t, "245", "5", 8, false)
	plain := modularTestInterval(t, "2", "10", 8, true)
	if !full.IsFull() || full.IsWrapped() || !wrapped.IsWrapped() || plain.IsSingleton() {
		t.Fatal("predicates")
	}
	if ok, err := inner.Implies(wrapped); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := wrapped.Implies(inner); err != nil || ok {
		t.Fatal(ok, err)
	}
	if ok, err := plain.Implies(wrapped); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if ok, err := wrapped.Implies(full); err != nil || !ok {
		t.Fatal(ok, err)
	}
	single := modularTestInterval(t, "7", "7", 8, true)
	if !single.IsSingleton() {
		t.Fatal("singleton")
	}
}
