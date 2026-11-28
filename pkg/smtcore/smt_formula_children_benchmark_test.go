package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Rebuild one scoped gate over surviving atoms. Each child goes through
// recursive Internalize even though it already has a Boolean variable.
func BenchmarkSMTInternalizeFormulaChildren(b *testing.B) {
	for _, width := range []int{2, 32} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			m := astTestManager(b)
			_, _, state := smtInternalizerFixture(b, m, nil)
			args := make([]ast.ASTHandle, width)
			for i := range args {
				args[i] = smtRelevancyBool(b, m, "arg"+strconv.Itoa(i))
				astTestMust(b, state.Internalize(args[i], true))
			}
			gate, err := m.MkOrArray(args)
			astTestMust(b, err)
			astTestPin(b, m, gate)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := state.PushScope(); err != nil {
					b.Fatal(err)
				}
				if err := state.Internalize(gate, true); err != nil {
					b.Fatal(err)
				}
				if err := state.PopScopes(1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
