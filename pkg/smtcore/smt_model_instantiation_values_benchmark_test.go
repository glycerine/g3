package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// The restricted auxiliary model checker enumerates the inverse map's keys
// once per binder when it builds the disjunction of allowed Skolem values.
func BenchmarkSMTInstantiationValues(b *testing.B) {
	for _, size := range []int{1, 8, 64} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			m := astTestManager(b)
			sort, err := m.MkUninterpretedSort(astTestSymbol(b, m, "values"), nil)
			astTestMust(b, err)
			set, err := newSMTModelFinderInstantiationSet(m)
			astTestMust(b, err)
			b.Cleanup(func() { astTestMust(b, set.Close()) })
			for i := 0; i < size; i++ {
				term, err := m.MkFreshConst("key", sort)
				astTestMust(b, err)
				_, err = set.Insert(term, 0)
				astTestMust(b, err)
			}
			astTestMust(b, set.MakeInverse(func(h ast.ASTHandle) (ast.ASTHandle, error) {
				return h, nil
			}))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				values, err := set.Values()
				if err != nil || len(values) != size {
					b.Fatalf("got %d values, want %d, err %v", len(values), size, err)
				}
			}
		})
	}
}
