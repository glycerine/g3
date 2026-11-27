package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Capture the same number of candidate tuples, then reset their references
// together as the model checker does after publishing a restart batch.
func BenchmarkSMTModelCheckerInstancePins(b *testing.B) {
	for _, width := range []int{1, 2, 8} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			m, state, _, _ := smtQuantifierManagerFixture(b, 10, 20, SMTQuantifierManagerConfig{})
			c, err := NewSMTModelChecker(state, 1)
			astTestMust(b, err)
			b.Cleanup(func() { astTestMust(b, c.Close()) })
			boolean, err := m.MkBoolSort()
			astTestMust(b, err)
			truth, err := m.MkTrue()
			astTestMust(b, err)
			sorts := make([]ast.ASTHandle, width)
			names := make([]symbol.Symbol, width)
			args := make([]ast.ASTHandle, width)
			for i := range args {
				sorts[i] = boolean
				names[i] = astTestSymbol(b, m, fmt.Sprint("x", i))
				args[i] = truth
			}
			body, err := m.MkVar(0, boolean)
			astTestMust(b, err)
			q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
			astTestMust(b, err)
			astTestPin(b, m, q)
			astTestMust(b, state.Internalize(q, true))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j := 0; j < 16; j++ {
					if _, err := c.enqueueCounterexample(q, args, true, 0); err != nil {
						b.Fatal(err)
					}
				}
				if err := c.ResetNewInstances(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(16, "instances/op")
		})
	}
}
