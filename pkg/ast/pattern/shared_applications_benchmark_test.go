package pattern

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Two applications each have width children. The conjunction references the
// same two applications width times. Its DAG has 3*width argument edges.
func BenchmarkPatternSharedApplications(b *testing.B) {
	for _, width := range []int{4, 16, 128, 512} {
		b.Run(fmt.Sprintf("Width%d", width), func(b *testing.B) {
			m := patternManager(b)
			pin := func(h ast.ASTHandle, e error) ast.ASTHandle {
				b.Helper()
				patternMust(b, e)
				patternMust(b, m.IncRef(h))
				b.Cleanup(func() { patternMust(b, m.DecRef(h)) })
				return h
			}
			boolean := pin(m.MkBoolSort())
			x := pin(m.MkVar(0, boolean))
			y := pin(m.MkVar(1, boolean))
			sorts := make([]ast.ASTHandle, width)
			args := make([]ast.ASTHandle, width)
			for i := range args {
				sorts[i] = boolean
				args[i] = x
				if i%2 != 0 {
					args[i] = y
				}
			}
			var terms [2]ast.ASTHandle
			for i := range terms {
				f := pin(m.MkFuncDecl(symbol.Numerical(uint32(900+i)), sorts, boolean, ast.ASTFuncDeclInfo{}))
				terms[i] = pin(m.MkApp(f, args))
			}
			for i := range args {
				args[i] = terms[i%2]
			}
			body := pin(m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, args))
			q := pin(m.MkQuantifier(ast.ASTQuantifierSpec{
				Forall: true, Sorts: []ast.ASTHandle{boolean, boolean},
				Names: []symbol.Symbol{symbol.Numerical(0), symbol.Numerical(1)},
				Body:  body, Weight: 1,
			}))
			c, err := newPatternCandidates(m, 2, nil, false)
			patternMust(b, err)
			b.Cleanup(func() { patternMust(b, c.close()) })
			opts := quantifierInferenceOptions{noPatternWeight: -1}
			reduce := func() ast.ASTRef {
				out, changed, err := c.reduceQuantifierNoDatabase(q, body, nil, opts)
				if err != nil || !changed {
					b.Fatalf("pattern inference returned %t, %v", changed, err)
				}
				return out
			}
			warm := reduce()
			inferred, err := m.RefGet(warm)
			patternMust(b, err)
			n, err := m.GetNumPatterns(inferred)
			patternMust(b, err)
			if n != 2 {
				b.Fatalf("got %d patterns", n)
			}
			for j, want := range terms {
				p, err := m.GetPattern(inferred, uint32(j))
				patternMust(b, err)
				h, err := m.GetArg(p, 0)
				patternMust(b, err)
				if h != want {
					b.Fatal("inferred terms are out of order")
				}
			}
			patternMust(b, m.DelRef(&warm))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r := reduce()
				if err = m.DelRef(&r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
