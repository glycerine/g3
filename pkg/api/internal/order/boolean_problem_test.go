package order

import "testing"

func TestBitCompilerGateIdentities(t *testing.T) {
	k := bitCompiler{BooleanProblem: BooleanProblem{Vars: 2*bitCompilerLimit - 8}, gates: make(map[uint64]int32)}
	high := int32(k.Vars)
	gates := []bitGate{
		{2, high, 0}, {-2, high, 0}, {2, -high, 0},
		{2, high, 3}, {2, high, -3}, {2, 3, high}, {high, 3, 2},
	}
	for i, g := range gates {
		want := high + 1 + int32(i)
		if got := k.intern(g); got != want {
			t.Fatalf("intern(%v) = %d, want %d", g, got, want)
		}
	}
	for i, g := range gates {
		if got := k.intern(g); got != high+1+int32(i) {
			t.Fatalf("repeated gate %v returned %d", g, got)
		}
	}
}

func TestBitCompilerRefusesOversizedCircuit(t *testing.T) {
	k := bitCompiler{
		BooleanProblem: BooleanProblem{Vars: 2*bitCompilerLimit - 1},
		ceiling:        500_000,
		nodes:          make([]bitGate, bitCompilerLimit-1, bitCompilerLimit),
		gates:          make(map[uint64]int32),
	}
	last := bitGate{2, 3, 0}
	x := k.intern(last)
	if x != 2*bitCompilerLimit || k.intern(last) != x || k.work != 0 {
		t.Fatal("last gate should fit and remain reusable")
	}
	k.intern(bitGate{2, -3, 0})
	if k.charge(0) != errBitLimit || k.Vars != 2*bitCompilerLimit || len(k.nodes) != bitCompilerLimit {
		t.Fatal("overflowed circuit was not rejected")
	}
}
