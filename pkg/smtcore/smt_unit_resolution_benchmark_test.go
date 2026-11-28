package smtcore

import (
	"strconv"
	"testing"
)

// One or two false literals removed from a ternary gate clause become the
// antecedents of its unit-resolution reason. Keep a window of reasons live.
func BenchmarkSMTUnitResolutionReason(b *testing.B) {
	literals := []SMTLiteral{2, 4, 6}
	for n := 1; n <= len(literals); n++ {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var live [64]*SMTUnitResolutionJustification
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				j, err := NewSMTUnitResolutionJustification(true, nil, literals[:n])
				if err != nil {
					b.Fatal(err)
				}
				live[i%len(live)] = j
			}
			if got := live[(b.N-1)%len(live)].Literals; len(got) != n || got[n-1] != literals[n-1] {
				b.Fatal("incorrect reason literals", got)
			}
		})
	}
}
