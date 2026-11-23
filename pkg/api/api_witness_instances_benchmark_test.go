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
