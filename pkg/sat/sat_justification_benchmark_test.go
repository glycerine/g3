package sat

import (
	"testing"
	"unsafe"
)

// Exercise both fast binary reasons and allocator-relative clause locators.
func BenchmarkSATJustificationColumn(b *testing.B) {
	for _, tc := range []struct {
		name        string
		clauseEvery uint32
	}{
		{"Binary", 0},
		{"Mixed", 8},
		{"Clause", 1},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var column satSolverJustifications
			column.bind()
			column.clauses = &ClauseAllocator{origin: 91}
			const size = 16384
			for i := 0; i < size; i++ {
				if err := column.push(satJustification{}); err != nil {
					b.Fatal(err)
				}
			}
			b.Cleanup(func() {
				if err := column.finalize(); err != nil {
					b.Fatal(err)
				}
			})
			b.ReportAllocs()
			b.ResetTimer()
			for j := 0; j < b.N; j++ {
				i := uint32(j) * 4271 & (size - 1)
				value := newSATBinaryJustification(Literal(i))
				if tc.clauseEvery != 0 && i%tc.clauseEvery == 0 {
					value = newSATClauseJustification(satClauseOffset{
						owner: 91, slot: i + 1, generation: 1 + i%7, storageKind: i & 1,
					})
				}
				if err := column.write(i, value); err != nil {
					b.Fatal(err)
				}
				got, err := column.read(i)
				if err != nil || got != value {
					b.Fatalf("slot %d returned %v, %v", i, got, err)
				}
			}
			b.ReportMetric(float64(unsafe.Sizeof(column.values[0])), "bytes/row")
		})
	}
}
