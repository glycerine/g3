package tactic

// Source: src/tactic/probe.cpp:58-208,426-460.

import (
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestProbeArithmeticComparisonBooleanAndOwnership(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	b := m.Budget()
	left, err := NewProbe(b, ProbeConstant, 6)
	astTestMust(t, err)
	right, err := NewProbe(b, ProbeConstant, 2)
	astTestMust(t, err)

	tests := []struct {
		kind ProbeKind
		want float64
	}{
		{ProbeAnd, 1}, {ProbeOr, 1}, {ProbeImplies, 1},
		{ProbeEqual, 0}, {ProbeNotEqual, 1},
		{ProbeLessEqual, 0}, {ProbeLess, 0},
		{ProbeGreaterEqual, 1}, {ProbeGreater, 1},
		{ProbeAdd, 8}, {ProbeSubtract, 4}, {ProbeMultiply, 12}, {ProbeDivide, 3},
	}
	roots := make([]*Probe, 0, len(tests)+1)
	for _, tc := range tests {
		p, err := NewBinaryProbe(b, tc.kind, left, right)
		astTestMust(t, err)
		roots = append(roots, p)
		got, err := p.Eval(g)
		astTestMust(t, err)
		if got != tc.want {
			t.Fatalf("kind %d: got %v want %v", tc.kind, got, tc.want)
		}
	}
	not, err := NewNotProbe(b, left)
	astTestMust(t, err)
	roots = append(roots, not)
	got, err := not.Eval(g)
	astTestMust(t, err)
	if got != 0 {
		t.Fatal("not probe", got)
	}
	if left.References != uint32(len(roots)) || right.References != uint32(len(tests)) {
		t.Fatal("composed probes did not retain children", left.References, right.References)
	}
	for _, p := range roots {
		astTestMust(t, p.Close())
	}
	if !left.Closed || !right.Closed || left.References != 0 || right.References != 0 {
		t.Fatal("composed probes did not release children")
	}

	zero, err := NewProbe(b, ProbeConstant, 0)
	astTestMust(t, err)
	one, err := NewProbe(b, ProbeConstant, 1)
	astTestMust(t, err)
	and, err := NewBinaryProbe(b, ProbeAnd, zero, one)
	astTestMust(t, err)
	got, err = and.Eval(g)
	astTestMust(t, err)
	if got != 0 || math.Signbit(got) {
		t.Fatal("false conjunction", got)
	}
	astTestMust(t, and.Close())
}

func TestProbeGoalProductionConfiguration(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	g, err := NewGoal(m, true, false, true)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, g.Close()) })
	tests := []struct {
		kind ProbeKind
		want float64
	}{
		{ProbeProduceProofs, 1},
		{ProbeProduceModels, 0},
		{ProbeProduceUnsatCores, 1},
	}
	for _, tc := range tests {
		p, err := NewProbe(m.Budget(), tc.kind, 0)
		astTestMust(t, err)
		got, err := p.Eval(g)
		astTestMust(t, err)
		if got != tc.want {
			t.Fatalf("kind %d: got %v want %v", tc.kind, got, tc.want)
		}
		astTestMust(t, p.Close())
	}
}

// AUDIT.md tactic_top_01-F2: C++ encodes lt/gt as not(le), so a NaN operand
// makes both true, while le/ge are false.
func TestProbeComparisonsWithNaNMatchSourceEncoding(t *testing.T) {
	m := astTestManager(t)
	g := goalTestNew(t, m, false, false)
	b := m.Budget()
	z1, err := NewProbe(b, ProbeConstant, 0)
	astTestMust(t, err)
	z2, err := NewProbe(b, ProbeConstant, 0)
	astTestMust(t, err)
	nan, err := NewBinaryProbe(b, ProbeDivide, z1, z2)
	astTestMust(t, err)
	one, err := NewProbe(b, ProbeConstant, 1)
	astTestMust(t, err)
	tests := []struct {
		kind ProbeKind
		want float64
	}{
		{ProbeLess, 1}, {ProbeGreater, 1}, {ProbeLessEqual, 0}, {ProbeGreaterEqual, 0},
	}
	roots := []*Probe{}
	for _, tc := range tests {
		p, err := NewBinaryProbe(b, tc.kind, nan, one)
		astTestMust(t, err)
		roots = append(roots, p)
		got, err := p.Eval(g)
		astTestMust(t, err)
		if got != tc.want {
			t.Fatalf("kind %d: got %v want %v", tc.kind, got, tc.want)
		}
	}
	for _, p := range roots {
		astTestMust(t, p.Close())
	}
	if !nan.Closed || !one.Closed || !z1.Closed || !z2.Closed {
		t.Fatal("NaN comparison probes did not release children")
	}
}
