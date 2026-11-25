package smtcore

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

// Revisit expressions after construction, as happens when separate clauses
// and quantifier instances share terms. Keep the nodes and Boolean variables
// live and verify that revisiting creates no new solver state.
func BenchmarkSMTRepeatedInternalization(b *testing.B) {
	f := newSMTEqualityUndoFixture(b, "revisit", 8)
	atom := smtRelevancyBool(b, f.m, "revisited-atom")
	negative, e := f.m.MkNot(atom)
	astTestMust(b, e)
	astTestPin(b, f.m, negative)
	pair, e := f.m.MkApp(f.binary, []ast.ASTHandle{f.base[0], f.base[1]})
	astTestMust(b, e)
	astTestPin(b, f.m, pair)
	for _, test := range []struct {
		name   string
		handle ast.ASTHandle
		gate   bool
	}{
		{"VariableInGate", atom, true}, {"VariableAsTerm", atom, false}, {"NegatedGate", negative, true}, {"EqualityGraphTerm", pair, false},
	} {
		b.Run(test.name, func(b *testing.B) {
			astTestMust(b, f.s.Internalize(test.handle, test.gate))
			vars, nodes := len(f.s.boolExpr), len(f.s.enodeStack)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := f.s.Internalize(test.handle, test.gate); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			if len(f.s.boolExpr) != vars || len(f.s.enodeStack) != nodes {
				b.Fatal("revisiting the expression created solver state")
			}
		})
	}
}
