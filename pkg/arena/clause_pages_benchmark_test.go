package arena

import (
	"encoding/binary"
	"math/rand"
	"testing"
)

// The saved view is an optimistic comparison for a read with known lifetime.
// Both probes consume the same header word in the same shuffled record order.
func BenchmarkPhysicalRowLookup(b *testing.B) {
	type record struct {
		ref  SmallBytesRef
		view []byte
	}
	for _, cached := range []bool{false, true} {
		name := "Lookup"
		if cached {
			name = "SavedView"
		}
		b.Run(name, func(b *testing.B) {
			a, err := NewClauseBytes(SmallBytesConfig{})
			if err != nil {
				b.Fatal(err)
			}
			defer a.Close()
			records := make([]record, 16384)
			var expected uint64
			for i := range records {
				ref, err := a.Allocate(uint32(36 + 4*(i%25)))
				if err != nil {
					b.Fatal(err)
				}
				row, err := a.Borrow(ref)
				if err != nil {
					b.Fatal(err)
				}
				binary.LittleEndian.PutUint32(row, uint32(i+1))
				records[i] = record{ref, row}
				expected += uint64(i + 1)
			}
			rng := rand.New(rand.NewSource(101))
			rng.Shuffle(len(records), func(i, j int) { records[i], records[j] = records[j], records[i] })
			var sum uint64
			b.ResetTimer()
			for k := 0; k < b.N; k++ {
				if cached {
					for i := range records {
						sum += uint64(binary.LittleEndian.Uint32(records[i].view))
					}
				} else {
					for i := range records {
						view, err := a.Borrow(records[i].ref)
						if err != nil {
							b.Fatal(err)
						}
						sum += uint64(binary.LittleEndian.Uint32(view))
					}
				}
			}
			b.StopTimer()
			if sum != expected*uint64(b.N) {
				b.Fatal(sum)
			}
			b.ReportMetric(float64(len(records)), "reads/op")
		})
	}
}
