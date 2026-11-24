package smtcore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Count incremental live heap with 512 enodes and different numbers of distinct
// declarations. All term DAGs are built and retained before the first sample.
func BenchmarkSMTDeclarationPopulation(b *testing.B) {
	const workers, width = 8, 512
	for _, mode := range []string{"One", "SixtyFour", "Singletons", "Constants", "Suppressed"} {
		b.Run(mode, func(b *testing.B) {
			type worker struct {
				f     *smtEqualityUndoFixture
				terms [width]ast.ASTHandle
			}
			var ws [workers]worker
			declCount := 1
			switch mode {
			case "SixtyFour", "Suppressed":
				declCount = 64
			case "Singletons":
				declCount = width
			}
			var active, idle int64
			b.ReportAllocs()
			b.ResetTimer()
			b.StopTimer()
			for rep := 0; rep < b.N; rep++ {
				for wi := range ws {
					f := newSMTEqualityUndoFixture(b, "decl-pop", 32)
					ws[wi].f = f
					ds := make([]ast.ASTHandle, declCount)
					if mode != "Constants" {
						for i := range ds {
							decl, err := f.m.MkFuncDecl(astTestSymbol(b, f.m, fmt.Sprintf("decl-pop-f%d", i)), []ast.ASTHandle{f.sort, f.sort}, f.sort, ast.ASTFuncDeclInfo{})
							astTestMust(b, err)
							ds[i] = decl
						}
					}
					for i := range width {
						var term ast.ASTHandle
						var err error
						if mode == "Constants" {
							term, err = f.m.MkConst(astTestSymbol(b, f.m, fmt.Sprintf("decl-pop-k%d", i)), f.sort)
						} else {
							term, err = f.m.MkApp(ds[i%declCount], []ast.ASTHandle{f.base[i%32], f.base[i/32]})
						}
						astTestMust(b, err)
						astTestPin(b, f.m, term)
						ws[wi].terms[i] = term
					}
					astTestMust(b, f.s.PushScope())
				}
				base := scopeBenchmarkLiveHeap()
				b.StartTimer()
				for wi := range ws {
					w := &ws[wi]
					for j, t := range w.terms {
						if mode == "Suppressed" && (j/declCount)%2 == 0 {
							_, err := w.f.s.MkEnode(t, true, false, true)
							astTestMust(b, err)
						} else {
							astTestMust(b, w.f.s.Internalize(t, false))
						}
					}
				}
				b.StopTimer()
				active += scopeBenchmarkLiveHeap() - base
				for wi := range ws {
					w := &ws[wi]
					for _, t := range w.terms {
						if w.f.s.context.enodes.get(t) == nil {
							b.Fatal("missing live term")
						}
					}
				}
				b.StartTimer()
				for wi := range ws {
					astTestMust(b, ws[wi].f.s.PopScopes(1))
				}
				b.StopTimer()
				idle += scopeBenchmarkLiveHeap() - base
				for wi := range ws {
					w := &ws[wi]
					for _, t := range w.terms {
						if w.f.s.context.enodes.get(t) != nil {
							b.Fatal("retired term still registered")
						}
					}
					astTestMust(b, w.f.s.Close())
				}
			}
			b.ReportMetric(float64(active)/float64(b.N*workers), "peak-B/solver")
			b.ReportMetric(float64(idle)/float64(b.N*workers), "idle-B/solver")
			runtime.KeepAlive(ws)
		})
	}
}
