package api

import "testing"

// A fresh strategic solver builds its tactics on the first assertion. Keep the
// context, asserted term and options alive across independent solver lifetimes.
func BenchmarkStrategicSolverFirstAssertion(b *testing.B) {
	benchmarkStrategicSolverFirstAssertion(b, (*SMTContext).NewSolver)
}

func BenchmarkIncrementalStrategicSolverFirstAssertion(b *testing.B) {
	benchmarkStrategicSolverFirstAssertion(b, (*SMTContext).NewSolverForIncrementalChecks)
}

func benchmarkStrategicSolverFirstAssertion(b *testing.B, newSolver func(*SMTContext) (*APISimpleSolver, error)) {
	for _, mode := range []string{"Default", "MacroFinder"} {
		b.Run(mode, func(b *testing.B) {
			ctx, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := ctx.Close(); err != nil {
					b.Fatal(err)
				}
			}()
			m, err := ctx.Manager()
			if err != nil {
				b.Fatal(err)
			}
			truth, err := m.MkTrue()
			if err != nil {
				b.Fatal(err)
			}
			if err := m.IncRef(truth); err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := m.DecRef(truth); err != nil {
					b.Fatal(err)
				}
			}()
			var options *ParameterSet
			if mode == "MacroFinder" {
				options, err = ctx.NewParameterSet()
				if err != nil {
					b.Fatal(err)
				}
				defer func() {
					if err := options.Close(); err != nil {
						b.Fatal(err)
					}
				}()
				if err := options.SetBool("smt.macro_finder", true); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s, err := newSolver(ctx)
				if err != nil {
					b.Fatal(err)
				}
				if options != nil {
					if err := s.SetParameters(options); err != nil {
						b.Fatal(err)
					}
				}
				if err := s.Assert(truth); err != nil {
					b.Fatal(err)
				}
				if err := s.Close(); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
		})
	}
}
