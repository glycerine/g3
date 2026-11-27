package pattern

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Measure the first pattern-inference pass on 64 distinct universal axioms.
// Each body has an eight-deep application chain containing its bound variable.
func BenchmarkInferenceQuantifierSubtrees(b *testing.B) {
	m := patternManager(b)
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		b.Helper()
		patternMust(b, err)
		patternMust(b, m.IncRef(h))
		b.Cleanup(func() { patternMust(b, m.DecRef(h)) })
		return h
	}
	boolean := pin(m.MkBoolSort())
	x := pin(m.MkVar(0, boolean))
	rows := make([]ast.ASTHandle, 64)
	for i := range rows {
		f := pin(m.MkFuncDecl(symbol.Numerical(uint32(100+i)), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
		term := x
		for j := 0; j < 8; j++ {
			term = pin(m.MkApp(f, []ast.ASTHandle{term}))
		}
		rows[i] = pin(m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: term, Weight: 1}))
	}
	for _, name := range []string{"Full", "QuantifierSubtrees"} {
		b.Run(name, func(b *testing.B) {
			out, err := m.NewRef(ast.ASTHandle{})
			patternMust(b, err)
			b.Cleanup(func() { patternMust(b, m.DelRef(&out)) })
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				r, err := NewInferenceRewriter(m)
				patternMust(b, err)
				rewrite := r.Rewrite
				if name == "QuantifierSubtrees" {
					rewrite = r.RewriteQuantifierSubtrees
				}
				for _, q := range rows {
					if err := rewrite(q, out); err != nil {
						b.Fatal(err)
					}
				}
				patternMust(b, r.Close())
			}
		})
	}
}
