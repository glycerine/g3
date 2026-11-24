package smtcore

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Measure only PopScopes. The caller retains every application, none, every
// other application, or all except every 29th application, respectively.
func BenchmarkScopedBooleanLifetimes(b *testing.B) {
	for _, size := range []int{128, 2048, 8192} {
		for _, pattern := range []string{"retained", "last", "mixed", "rare"} {
			b.Run(fmt.Sprintf("%d/%s", size, pattern), func(b *testing.B) {
				m := astTestManager(b)
				_, _, s := smtInternalizerFixture(b, m, nil)
				benchmarkMust(b, s.ensureDelayedActivityQueue())
				var held []ast.ASTHandle
				sort, err := m.MkBoolSort()
				benchmarkMust(b, err)
				astTestPin(b, m, sort)
				syms := make([]symbol.Symbol, size)
				for i := range syms {
					syms[i] = astTestSymbol(b, m, "bench-life-"+strconv.Itoa(i))
				}
				b.ReportAllocs()
				b.ResetTimer()
				b.StopTimer()
				for k := 0; k < b.N; k++ {
					benchmarkMust(b, s.PushScope())
					held = held[:0]
					for i, sym := range syms {
						h, e := m.MkConst(sym, sort)
						benchmarkMust(b, e)
						if pattern == "retained" || pattern == "mixed" && i%2 == 0 || pattern == "rare" && i%29 != 0 {
							benchmarkMust(b, m.IncRef(h))
							held = append(held, h)
						}
						_, e = s.MkBoolVar(h)
						benchmarkMust(b, e)
					}
					count := s.statistics.NumDelBoolVar
					b.StartTimer()
					err = s.PopScopes(1)
					b.StopTimer()
					benchmarkMust(b, err)
					if s.statistics.NumDelBoolVar-count != uint32(size) {
						b.Fatal("retired variable count", s.statistics.NumDelBoolVar-count)
					}
					for _, h := range held {
						benchmarkMust(b, m.DecRef(h))
					}
				}
			})
		}
	}
}
