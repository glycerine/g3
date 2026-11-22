package api

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/solver"
)

func BenchmarkSimpleSolverAssertionSnapshot(b *testing.B) {
	for _, n := range []int{2, 32, 256, 1024} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			ctx, err := NewSMTContext()
			if err != nil {
				b.Fatal(err)
			}
			defer ctx.Close()
			s, err := ctx.NewSimpleSolver()
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			var input strings.Builder
			for j := 0; j < n; j++ {
				fmt.Fprintf(&input, "(declare-const p%d Bool)(assert p%d)\n", j, j)
			}
			if err := s.LoadSMT2String(input.String()); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				assertions, err := solver.Assertions(s.solver)
				if err != nil || len(assertions) != n {
					b.Fatalf("snapshot returned %d assertions, %v", len(assertions), err)
				}
			}
			b.StopTimer()
		})
	}
}
