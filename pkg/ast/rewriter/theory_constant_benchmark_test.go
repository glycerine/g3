package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func BenchmarkTheoryConstantRewrite(b *testing.B) {
	m, c, g, p := boolRewriterConfigFixture(b)
	r, err := NewTheoryRewriter(m, c, g, p)
	astTestMust(b, err)
	b.Cleanup(func() { astTestMust(b, r.Close()) })
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	atom, err := m.MkConst(astTestSymbol(b, m, "constant"), sort)
	astTestMust(b, err)
	astTestPin(b, m, atom)
	truth, err := m.MkTrue()
	astTestMust(b, err)
	astTestPin(b, m, truth)
	out := astTestPin(b, m, ast.ASTHandle{})
	for _, tc := range []struct {
		name string
		term ast.ASTHandle
	}{{"Uninterpreted", atom}, {"True", truth}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := r.Rewrite(tc.term, out); err != nil {
					b.Fatal(err)
				}
			}
			got, err := m.RefGet(out)
			astTestMust(b, err)
			if got != tc.term {
				b.Fatalf("got %v, want %v", got, tc.term)
			}
		})
	}
}
