package arena

import (
	"strconv"
	"testing"
)

// A 256-byte allocation owns one chunk. Compare directory growth with reuse
// of either dispersed holes or holes at the end of an occupied prefix.
func BenchmarkSmallBytesChunkDirectory(b *testing.B) {
	const batch = 64
	for _, count := range []int{512, 4096, 16384} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			for _, shape := range []string{"Grow", "SparseReuse", "LateHoles"} {
				b.Run(shape, func(b *testing.B) {
					store, err := NewSmallBytes(SmallBytesConfig{})
					if err != nil {
						b.Fatal(err)
					}
					defer store.Close()
					refs := make([]SmallBytesRef, count)
					fill := func() {
						for i := range refs {
							r, err := store.Allocate(256)
							if err != nil {
								b.Fatal(err)
							}
							refs[i] = r
						}
					}
					if shape != "Grow" {
						fill()
					}
					b.ReportAllocs()
					b.ResetTimer()
					for round := 0; round < b.N; round++ {
						if shape == "Grow" {
							fill()
							store.Reset()
							continue
						}
						var indices [batch]int
						for j := range indices {
							// The odd stride permutes a power-of-two working set.
							i := ((round*batch + j) * 2377) & (count - 1)
							if shape == "LateHoles" {
								i = count - batch + j
							}
							indices[j] = i
							if err := store.Deallocate(256, refs[i]); err != nil {
								b.Fatal(err)
							}
						}
						for _, i := range indices {
							r, err := store.Allocate(256)
							if err != nil {
								b.Fatal(err)
							}
							refs[i] = r
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(count), "population")
				})
			}
		})
	}
}
