package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// An auxiliary solver can touch only a small subset of its manager's slots.
// Compare first writes in such a solver with a fully populated table.
func BenchmarkSMTExprTable(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	benchmarkMust(b, err)
	const count = 8192
	handles := make([]ast.ASTHandle, count)
	for i := range handles {
		handles[i], err = m.MkVar(uint32(i), sort)
		benchmarkMust(b, err)
	}
	for _, tc := range []struct {
		name   string
		stride int
	}{{"Dense", 1}, {"Sparse", 1024}} {
		var keys []ast.ASTHandle
		for i := 0; i < count; i += tc.stride {
			keys = append(keys, handles[i])
		}
		b.Run("Fill/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var table smtExprTable
				bools := smtBoolVarTable{&table}
				for j, key := range keys {
					bools.set(key, int32(j))
				}
				if bools.count() != len(keys) {
					b.Fatal("lost Boolean variables", bools.count())
				}
			}
		})
		b.Run("Lookup/"+tc.name, func(b *testing.B) {
			var table smtExprTable
			bools := smtBoolVarTable{&table}
			for j, key := range keys {
				bools.set(key, int32(j))
			}
			mask := len(keys) - 1
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				j := i * 37 & mask
				if variable, found := bools.lookup(keys[j]); !found || variable != int32(j) {
					b.Fatal("unexpected Boolean variable", variable, found)
				}
			}
		})
	}
}
