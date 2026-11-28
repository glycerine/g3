package smtcore

import (
	"fmt"
	"testing"
)

// Two assigned literals explain the conflict. The remaining internalized
// variables measure the cost of unrelated solver state during resolution.
func BenchmarkSMTBinaryConflictLevels(b *testing.B) {
	benchmarkSMTConflictWidths(b, []int{64, 1024, 8192}, false)
}

// A nonterminal conflict also marks the literals used to learn its clause.
func BenchmarkSMTLearnedConflictMarks(b *testing.B) {
	benchmarkSMTConflictWidths(b, []int{64, 1024, 8192, 65536}, true)
}

func benchmarkSMTConflictWidths(b *testing.B, widths []int, learned bool) {
	for _, width := range widths {
		b.Run(fmt.Sprintf("Vars%d", width), func(b *testing.B) {
			w := newScopeBenchmarkWorker(b, width)
			for i, expr := range w.expressions {
				benchmarkMust(b, w.s.Internalize(expr, true))
				v, ok := w.s.BoolVar(expr)
				if !ok {
					b.Fatal("missing Boolean variable")
				}
				w.literals[i] = NewSMTLiteral(v, false)
			}
			a, z := w.literals[0], w.literals[width-1]
			if learned {
				benchmarkMust(b, w.s.PushScope())
			}
			benchmarkMust(b, w.s.AssignLiteral(a, NewSMTBooleanAxiom()))
			if learned {
				benchmarkMust(b, w.s.PushScope())
			}
			benchmarkMust(b, w.s.AssignLiteral(z, NewSMTBooleanAxiom()))
			r, err := NewSMTConflictResolver(w.s)
			benchmarkMust(b, err)
			b.Cleanup(func() { benchmarkMust(b, r.Close()) })
			reason := NewSMTBooleanBinaryClause(a)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := r.Resolve(reason, z, 0)
				if err != nil || result.Resolved != learned || learned && len(result.Lemma) != 2 {
					b.Fatalf("conflict resolved=%v, error=%v", result.Resolved, err)
				}
			}
		})
	}
}
