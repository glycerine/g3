package smtcore

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Many tuples share a quantifier. The first binding distinguishes them,
// while the remaining positions exercise argument storage and tuple hashing.
func BenchmarkSMTFingerprint(b *testing.B) {
	m := astTestManager(b)
	boolean, err := m.MkBoolSort()
	benchmarkMust(b, err)
	bound, err := m.MkVar(0, boolean)
	benchmarkMust(b, err)
	const count = 1024
	var nodes [count]*SMTEnode
	for i := range nodes {
		_, nodes[i] = smtEnodeConstant(b, m, uint32(10000+i), boolean, 0)
	}
	for _, width := range []int{1, 3, 6} {
		sorts := make([]ast.ASTHandle, width)
		names := make([]symbol.Symbol, width)
		for i := range sorts {
			sorts[i] = boolean
			names[i] = symbol.Numerical(uint32(i))
		}
		q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: bound})
		benchmarkMust(b, err)
		id, err := m.GetASTID(q)
		benchmarkMust(b, err)
		var tuples [count][6]*SMTEnode
		for i := range tuples {
			for j := 0; j < width; j++ {
				tuples[i][j] = nodes[(i+j*37)%count]
			}
		}
		for _, operation := range []string{"Contains", "ScopedInsert"} {
			b.Run(operation+"/Arity"+strconv.Itoa(width), func(b *testing.B) {
				set := NewSMTFingerprintSet()
				benchmarkMust(b, set.PushScope())
				for i := range tuples {
					if f, err := set.Insert(q, id, tuples[i][:width]); err != nil || f == nil {
						b.Fatal("initial insertion", i, err)
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				if operation == "Contains" {
					for i := 0; i < b.N; i++ {
						if ok, err := set.Contains(q, id, tuples[i%count][:width]); err != nil || !ok {
							b.Fatal("missing tuple", i, err)
						}
					}
					return
				}
				for i := 0; i < b.N; i++ {
					k := i % count
					if k == 0 {
						benchmarkMust(b, set.PopScopes(1))
						benchmarkMust(b, set.PushScope())
					}
					if f, err := set.Insert(q, id, tuples[k][:width]); err != nil || f == nil {
						b.Fatal("scoped insertion", i, err)
					}
				}
			})
		}
	}
}

// Measure the queue between matching and instance construction. Eager
// entries take the ordinary queue path without building any AST lemmas.
func BenchmarkSMTQIQueueBatch(b *testing.B) {
	m := astTestManager(b)
	q := smtRelevancyBool(b, m, "queue-benchmark")
	for _, size := range []int{32, 1024} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			queue := NewSMTQIQueueSchedule(m, 1, 2, false)
			entries := make([]SMTFingerprint, size)
			for i := range entries {
				entries[i].data = q
			}
			seen := 0
			consume := func(entry SMTQIQueueEntry) error {
				if entry.Fingerprint != &entries[seen] || !entry.Instantiated {
					b.Fatal("wrong eager entry", seen)
				}
				seen++
				return nil
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				seen = 0
				for j := range entries {
					if err := queue.Insert(&entries[j], 1, 0); err != nil {
						b.Fatal(err)
					}
				}
				if err := queue.instantiateNew(nil, consume, nil); err != nil || seen != size || queue.HasWork() {
					b.Fatal("incomplete batch", seen, err)
				}
			}
		})
	}
}
