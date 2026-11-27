package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/solver"
)

// Parsing and source construction precede timing. Each iteration owns a new
// private arena, translates the assertions and solves the generated ground query.
func BenchmarkAppleWitnessInstances(b *testing.B) {
	for _, name := range []string{"cf_live-130", "cf_live-56", "rfn2-7"} {
		b.Run(name, func(b *testing.B) {
			input := appleBenchmarkInput(b, name)
			benchmarkWitnessWidth(b, input, false, true)
		})
	}
}

// These three consecutive Flash goals share a transition. Each goal selects
// most of its quantified premises for a separate ground-witness search.
func BenchmarkGroundRelationVerification(b *testing.B) {
	benchmarkVerificationGoals(b, "test_vectors/flash2_relation_goals.smt2.gz", 3)
}

// The live checker reaches these obligations with an incremental scope already
// open. Replay each standalone assertion set at both witness search widths.
func BenchmarkRaftWitnessWidth(b *testing.B) {
	cases := []struct {
		name         string
		wide, narrow bool
	}{
		{"abstract-raftok-54", true, true},
		{"abstract-raftok-55", false, false},
		{"abstract-raftok-100", true, true},
		{"abstract-raftok-373", true, false},
	}
	for _, tc := range cases {
		input := verificationBenchmarkInput(b, "ivy_raft", tc.name)
		b.Run(tc.name+"/wide", func(b *testing.B) { benchmarkWitnessWidth(b, input, false, tc.wide) })
		b.Run(tc.name+"/narrow", func(b *testing.B) { benchmarkWitnessWidth(b, input, true, tc.narrow) })
	}
}

func benchmarkWitnessWidth(b *testing.B, input string, narrow, wantProof bool) {
	b.Helper()
	c, err := NewSMTContext()
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	s, err := c.NewSolver()
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if err = s.LoadSMT2String(input); err != nil {
		b.Fatal(err)
	}
	assertions, err := solver.Assertions(s.solver)
	if err != nil {
		b.Fatal(err)
	}
	m, err := c.Manager()
	if err != nil {
		b.Fatal(err)
	}
	p := orderProofState{source: s}
	defer p.release()
	proofs := 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := p.proveGoalInstances(m, assertions, orderProofSteps, narrow)
		if err != nil {
			b.Fatal(err)
		}
		if got {
			proofs++
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(proofs)/float64(b.N), "proofs/op")
	if wantProof && proofs != b.N || !wantProof && proofs != 0 {
		b.Fatalf("refuted %d of %d obligations", proofs, b.N)
	}
}

// Both bounded witness searches return SAT on these unsatisfiable queries.
// The timed operation includes admission and any private proof work it starts.
func BenchmarkArithmeticWitnessAdmission(b *testing.B) {
	for _, name := range []string{"number-theory-2-25", "number-theory-2-30"} {
		b.Run(name, func(b *testing.B) {
			input := verificationBenchmarkInput(b, "ivy_arithmetic", name)
			benchmarkWitnessWidth(b, input, false, false)
		})
	}
}

// The wide private proof succeeds on batch 11 goal 6. A sibling batch can
// leave the same obligation to its incremental solver. Time the private attempt.
func BenchmarkSplitQueueWitnessSelection(b *testing.B) {
	input := verificationBenchmarkInput(b, "ivy_apple", "split-queue-11-6")
	for _, tc := range []struct {
		name  string
		goals int
	}{{"Unsized", 0}, {"SiblingBatch", 35}} {
		b.Run(tc.name, func(b *testing.B) {
			ctx, s := orderSolver(b, input)
			m, err := ctx.Manager()
			astTestMust(b, err)
			forms, err := solver.Assertions(s.solver)
			astTestMust(b, err)
			p := orderProofState{source: s, expectedGoals: tc.goals}
			b.Cleanup(func() { astTestMust(b, p.release()) })
			b.ReportAllocs()
			for b.Loop() {
				proved, err := p.proveGoalInstances(m, forms, orderProofSteps, false)
				if err != nil || proved != (tc.goals == 0) {
					b.Fatalf("private refutation = %v, error = %v", proved, err)
				}
			}
		})
	}
}
