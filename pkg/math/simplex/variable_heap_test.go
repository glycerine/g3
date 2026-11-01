package simplex

import "testing"

func TestSimplexVariableHeapSourceShapeAndIndexedErase(t *testing.T) {
	var h simplexVariableHeap
	for _, v := range []uint32{4, 1, 3, 0, 2} {
		h.insert(v)
	}
	wantShape := []uint32{0, 1, 3, 4, 2}
	for i, want := range wantShape {
		if h.values[i] != want || !h.contains(want) {
			t.Fatalf("source heap shape/position at %d: values=%v contains(%d)=%v", i, h.values, want, h.contains(want))
		}
	}
	h.erase(1)
	if h.contains(1) || h.indices[4] == 0 {
		t.Fatalf("indexed erase left stale membership: %v %v", h.values, h.indices)
	}
	for _, want := range []uint32{0, 2, 3, 4} {
		got, ok := h.popMin()
		if !ok || got != want {
			t.Fatalf("pop minimum = %d, %v; want %d", got, ok, want)
		}
	}
	if _, ok := h.popMin(); ok {
		t.Fatal("empty heap returned a value")
	}
}

func TestSimplexVariableHeapGrowthDuplicateAndClear(t *testing.T) {
	var h simplexVariableHeap
	h.insert(15)
	h.insert(15)
	if len(h.values) != 1 || !h.contains(15) {
		t.Fatalf("duplicate insertion: values=%v indices=%v", h.values, h.indices)
	}
	h.clear()
	if len(h.values) != 0 || h.contains(15) {
		t.Fatalf("clear left heap state: values=%v indices=%v", h.values, h.indices)
	}
	h.insert(7)
	if got, ok := h.popMin(); !ok || got != 7 {
		t.Fatalf("insert after clear: %d, %v", got, ok)
	}
}
