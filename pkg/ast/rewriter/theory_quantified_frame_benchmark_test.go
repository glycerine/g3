package rewriter

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// BenchmarkTheoryQuantifiedFrames rewrites a predicate over all bound variables
// of one universal quantifier, without supplying any external variable bindings.
func BenchmarkTheoryQuantifiedFrames(b *testing.B) {
	for _, width := range []int{1, 8, 64} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			m, c, g, p := boolRewriterConfigFixture(b)
			rw, err := NewTheoryRewriter(m, c, g, p)
			astTestMust(b, err)
			b.Cleanup(func() { astTestMust(b, rw.Close()) })

			sort, err := m.MkBoolSort()
			astTestMust(b, err)
			sorts := make([]ast.ASTHandle, width)
			names := make([]symbol.Symbol, width)
			args := make([]ast.ASTHandle, width)
			for i := range args {
				sorts[i] = sort
				names[i] = symbol.Numerical(uint32(i))
				args[i], err = m.MkVar(uint32(i), sort)
				astTestMust(b, err)
			}
			pred, err := m.MkFuncDecl(astTestSymbol(b, m, "P"), sorts, sort, ast.ASTFuncDeclInfo{})
			astTestMust(b, err)
			body, err := m.MkApp(pred, args)
			astTestMust(b, err)
			root, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: body})
			astTestMust(b, err)
			astTestPin(b, m, root)
			out := astTestPin(b, m, ast.ASTHandle{})
			astTestMust(b, rw.Rewrite(root, out))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := rw.Rewrite(root, out); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			got, err := m.RefGet(out)
			astTestMust(b, err)
			if got != root {
				b.Fatal("quantified predicate changed during rewriting")
			}
		})
	}
}
