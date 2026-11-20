package arena

import (
	"fmt"
	"testing"
)

// Each round frees 64 dispersed blocks, refills the same size class and
// checks the new payloads. The rest of the working set remains live.
func BenchmarkSmallBytesBlockReuse(b *testing.B) {
	const batch = 64
	for _, alignment := range []uint32{4, 8} {
		for _, size := range []uint32{3, 9, 56, 233} {
			for _, count := range []int{256, 8192} {
				b.Run(fmt.Sprintf("align%d/bytes%d/blocks%d", alignment, size, count), func(b *testing.B) {
					store, err := NewSmallBytes(SmallBytesConfig{Alignment: alignment})
					if err != nil {
						b.Fatal(err)
					}
					b.Cleanup(store.Close)
					refs := make([]SmallBytesRef, count)
					for i := range refs {
						refs[i], err = store.Allocate(size)
						if err != nil {
							b.Fatal(err)
						}
					}
					b.ReportAllocs()
					b.ResetTimer()
					for round := 0; round < b.N; round++ {
						var positions [batch]int
						for n := range positions {
							// An odd stride permutes the power-of-two working set.
							i := ((round*batch + n) * 25173) & (count - 1)
							positions[n] = i
							if err := store.Deallocate(size, refs[i]); err != nil {
								b.Fatal(err)
							}
						}
						for _, i := range positions {
							r, err := store.Allocate(size)
							if err != nil {
								b.Fatal(err)
							}
							refs[i] = r
							if err := store.Write(r, size-1, byte(i)); err != nil {
								b.Fatal(err)
							}
							value, err := store.Read(r, size-1)
							if err != nil || value != byte(i) {
								b.Fatal(value, err)
							}
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(b.Elapsed().Nanoseconds())/(float64(b.N)*batch), "ns/reuse")
					b.ReportMetric(float64(store.Stats().MetadataBytes), "metadata-B")
				})
			}
		}
	}
}
