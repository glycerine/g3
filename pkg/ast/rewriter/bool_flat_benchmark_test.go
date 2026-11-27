package rewriter

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Ticket frequently reduces an AND or OR of distinct signed operands to
// itself. Include widths on both sides of the small-operand cutoff.
func BenchmarkBoolFlatDistinctOperands(b *testing.B) {
	for _, n := range []int{2, 4, 8, 9, 32} {
		b.Run(fmt.Sprintf("width%d", n), func(b *testing.B) {
			m := astTestManager(b)
			rw := boolRewriter{manager: m, flat: true}
			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			args := make([]ast.ASTHandle, n)
			for i := range args {
				atom, err := m.MkConst(astTestSymbol(b, m, fmt.Sprintf("p%d", i)), sort)
				astTestMust(b, err)
				if i%2 == 1 {
					atom, err = m.MkNot(atom)
					astTestMust(b, err)
				}
				astTestPin(b, m, atom)
				args[i] = atom
			}
			out := astTestPin(b, m, args[0])
			b.ReportAllocs()
			for b.Loop() {
				status, err := rw.mkFlatOrCore(args, out)
				if status != brFailed || err != nil {
					b.Fatalf("unexpected reduction %v, %v", status, err)
				}
			}
		})
	}
}
