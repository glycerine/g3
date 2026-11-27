package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Each push internalizes fresh nodes for an already owned input DAG.
// One in sixteen applications lives below the scope and must survive.
func BenchmarkSMTEnodeScopeBatch(b *testing.B) {
	benchmarkSMTEnodeScopeBatch(b, false)
}

func BenchmarkSMTUnaryEnodeScopeBatch(b *testing.B) {
	benchmarkSMTEnodeScopeBatch(b, true)
}

func benchmarkSMTEnodeScopeBatch(b *testing.B, unary bool) {
	for _, width := range []int{32, 256, 2048} {
		b.Run(strconv.Itoa(width), func(b *testing.B) {
			roots := 64
			if unary {
				roots = width
			}
			f := newSMTEqualityUndoFixture(b, "scoped-enodes", roots)
			terms := make([]ast.ASTHandle, width)
			for i := range terms {
				decl := f.binary
				args := []ast.ASTHandle{f.base[i%roots], f.base[i/roots]}
				if unary {
					decl = f.unary
					args = args[:1]
				}
				t, err := f.m.MkApp(decl, args)
				astTestMust(b, err)
				astTestPin(b, f.m, t)
				terms[i] = t
				if i%16 == 0 {
					astTestMust(b, f.s.Internalize(t, false))
				}
			}
			base := len(f.s.enodeStack)
			b.ReportAllocs()
			b.ResetTimer()
			for k := 0; k < b.N; k++ {
				if err := f.s.PushScope(); err != nil {
					b.Fatal(err)
				}
				for i, t := range terms {
					if i%16 != 0 {
						if err := f.s.Internalize(t, false); err != nil {
							b.Fatal(err)
						}
					}
				}
				if len(f.s.enodeStack) != base+width-width/16 {
					b.Fatal("incomplete internalization")
				}
				if err := f.s.PopScopes(1); err != nil {
					b.Fatal(err)
				}
				if len(f.s.enodeStack) != base {
					b.Fatal("incomplete scope restoration")
				}
			}
			b.StopTimer()
			for i, t := range terms {
				if (f.s.context.enodes.get(t) != nil) != (i%16 == 0) {
					b.Fatalf("application %d has incorrect scope", i)
				}
			}
			b.ReportMetric(float64(width-width/16), "enodes/op")
		})
	}
}
