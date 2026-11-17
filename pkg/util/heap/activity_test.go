package heap

import (
	"math/rand"
	"testing"
)

// TestActivityHeapMatchesLessHeap drives an Activity heap and a heap with
// the equivalent Less closure through the same operations and compares
// every observable result.
func TestActivityHeapMatchesLessHeap(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	var act []float64
	const n = 300
	for i := 0; i < n; i++ {
		act = append(act, float64(r.Intn(40)))
	}
	a, err := New(n, Config{Activity: &act})
	heapMust(t, err)
	l, err := New(n, Config{Less: func(x, y int32) bool { return act[x] > act[y] }})
	heapMust(t, err)
	defer func() { heapMust(t, a.Close()); heapMust(t, l.Close()) }()
	for step := 0; step < 20000; step++ {
		v := int32(r.Intn(n))
		switch r.Intn(5) {
		case 0, 1:
			if !a.Contains(v) {
				heapMust(t, a.Insert(v))
				heapMust(t, l.Insert(v))
			}
		case 2:
			if !a.Empty() {
				x, e1 := a.EraseMin()
				y, e2 := l.EraseMin()
				heapMust(t, e1)
				heapMust(t, e2)
				if x != y {
					t.Fatal("EraseMin", step, x, y)
				}
			}
		case 3:
			if a.Contains(v) {
				act[v] += float64(r.Intn(5))
				heapMust(t, a.Decreased(v))
				heapMust(t, l.Decreased(v))
			}
		case 4:
			if a.Contains(v) {
				heapMust(t, a.Erase(v))
				heapMust(t, l.Erase(v))
			}
		}
		if a.Size() != l.Size() || !a.CheckInvariant() {
			t.Fatal("size/invariant", step)
		}
		for i := range a.values {
			if a.values[i] != l.values[i] {
				t.Fatal("values differ", step, i)
			}
		}
	}
}
