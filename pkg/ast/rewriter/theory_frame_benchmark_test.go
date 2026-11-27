package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// BenchmarkTheoryUnaryFrames rewrites a chain with a single owner for each
// intermediate application. Both cases use the same arity, depth and leaves.
func BenchmarkTheoryUnaryFrames(b *testing.B) {
	for _, name := range []string{"AlternatingDeclarations", "SameDeclaration"} {
		b.Run(name, func(b *testing.B) {
			m, c, g, p := boolRewriterConfigFixture(b)
			rw, err := NewTheoryRewriter(m, c, g, p)
			astTestMust(b, err)
			b.Cleanup(func() { astTestMust(b, rw.Close()) })
			hold := func(h ast.ASTHandle, err error) ast.ASTHandle {
				astTestMust(b, err)
				astTestPin(b, m, h)
				return h
			}
			s := hold(m.MkUninterpretedSort(astTestSymbol(b, m, "U"), nil))
			f := hold(m.MkFuncDecl(astTestSymbol(b, m, "f"), []ast.ASTHandle{s}, s, ast.ASTFuncDeclInfo{}))
			gDecl := hold(m.MkFuncDecl(astTestSymbol(b, m, "g"), []ast.ASTHandle{s}, s, ast.ASTFuncDeclInfo{}))
			root := hold(m.MkConst(astTestSymbol(b, m, "x"), s))
			for i := 0; i < 32; i++ {
				d := f
				if name == "AlternatingDeclarations" && i%2 == 1 {
					d = gDecl
				}
				root, err = m.MkApp(d, []ast.ASTHandle{root})
				astTestMust(b, err)
			}
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
				b.Fatalf("rewrote chain to %v, want %v", got, root)
			}
		})
	}
}
