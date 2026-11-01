package smtcore

import (
	"math/big"
	"testing"
)

func TestSMTRationalDiffHeapDecreaseAndReset(t *testing.T) {
	g := &SMTRationalDiffGraph{gamma: make([]big.Rat, 5)}
	g.heap.graph = g
	for variable, value := range []int64{9, 4, 0, 6, 7} {
		g.gamma[variable].SetInt64(value)
	}
	for _, variable := range []int32{4, 1, 3, 0} {
		g.heap.insert(variable)
	}
	g.gamma[0].SetInt64(-1)
	g.heap.decreased(0)
	for _, want := range []int32{0, 1, 3, 4} {
		if got := g.heap.eraseMin(); got != want {
			t.Fatalf("rational diff heap pop = %d, want %d", got, want)
		}
	}
	if !g.heap.empty() {
		t.Fatal("rational diff heap not empty after all pops")
	}
	for variable, position := range g.heap.positions {
		if position != -1 {
			t.Fatalf("rational diff heap left variable %d at position %d", variable, position)
		}
	}
	g.heap.insert(3)
	g.heap.reset()
	if !g.heap.empty() || g.heap.positions[3] != -1 {
		t.Fatalf("rational diff heap reset left state: %v %v", g.heap.variables, g.heap.positions)
	}
}

func TestSMTInfRationalDiffHeapDecreaseAndReset(t *testing.T) {
	g := &SMTInfRationalDiffGraph{gamma: make([]smtInfRational, 5)}
	g.heap.graph = g
	for variable, value := range []int64{9, 4, 0, 6, 7} {
		g.gamma[variable].rational.SetInt64(value)
	}
	for _, variable := range []int32{4, 1, 3, 0} {
		g.heap.insert(variable)
	}
	g.gamma[0].rational.SetInt64(-1)
	g.heap.decreased(0)
	for _, want := range []int32{0, 1, 3, 4} {
		if got := g.heap.eraseMin(); got != want {
			t.Fatalf("infinitesimal diff heap pop = %d, want %d", got, want)
		}
	}
	if !g.heap.empty() {
		t.Fatal("infinitesimal diff heap not empty after all pops")
	}
	g.heap.insert(3)
	g.heap.reset()
	if !g.heap.empty() || g.heap.positions[3] != -1 {
		t.Fatalf("infinitesimal diff heap reset left state: %v %v", g.heap.variables, g.heap.positions)
	}
}
