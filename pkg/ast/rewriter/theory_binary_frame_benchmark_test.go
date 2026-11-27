package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// BenchmarkTheoryBinaryFrames visits both operands of an unchanged binary
// application at each level. Every intermediate application has one parent.
func BenchmarkTheoryBinaryFrames(b *testing.B) {
	m, c, g, p := boolRewriterConfigFixture(b)
	rw, err := NewTheoryRewriter(m, c, g, p)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, rw.Close()) })
	hold := func(h ast.ASTHandle, err error) ast.ASTHandle {
		astTestMust(b, err)
		astTestPin(b, m, h)
		return h
	}
	sort := hold(m.MkUninterpretedSort(astTestSymbol(b, m, "U"), nil))
	f := hold(m.MkFuncDecl(astTestSymbol(b, m, "f"), []ast.ASTHandle{sort, sort}, sort, ast.ASTFuncDeclInfo{}))
	leaf := hold(m.MkConst(astTestSymbol(b, m, "x"), sort))
	root := leaf
	for i := 0; i < 32; i++ {
		root, err = m.MkApp(f, []ast.ASTHandle{root, leaf})
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
}
