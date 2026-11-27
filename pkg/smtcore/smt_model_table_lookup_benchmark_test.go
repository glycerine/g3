package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Use the ordinary evaluator with different hit positions and a default value.
// The work counter includes every row up to the first match.
func BenchmarkModelTableFunctionLookup(b *testing.B) {
	const rows = 32
	for _, arity := range []int{1, 2, tableMaxArity} {
		g := tableModel{
			terms:     []tableTerm{{}, {op: tableVariable}, {op: tableConstant, index: tableFalse}},
			sorts:     make([]ast.ASTHandle, rows+4),
			functions: []tableFunction{{fallback: 2, entries: make([]tableEntry, rows)}},
		}
		args := make([]int, arity)
		for i := range args {
			args[i] = 1
		}
		g.terms = append(g.terms, tableTerm{op: tableApplication, args: args})
		for i := range g.functions[0].entries {
			row := &g.functions[0].entries[i]
			for j := range args {
				row.keys[j] = i + 3
			}
			row.value = tableTrue
		}
		for _, tc := range []struct {
			name                string
			input, want, probes int
		}{
			{"First", 3, tableTrue, 1},
			{"Last", rows + 2, tableTrue, rows},
			{"Default", rows + 3, tableFalse, rows + 1}, // includes evaluating the fallback
		} {
			b.Run(fmt.Sprintf("Arity%d/%s", arity, tc.name), func(b *testing.B) {
				bindings := []int{tc.input}
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					g.work = 256
					value := g.evaluate(3, bindings, 0)
					if value != tc.want || g.incomplete || g.work != 256-1-arity-tc.probes {
						b.Fatalf("value %d, work left %d, incomplete %t", value, g.work, g.incomplete)
					}
				}
			})
		}
	}
}
