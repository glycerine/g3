package arena

import (
	"fmt"
	"testing"
)

func BenchmarkDescriptorReadLive(b *testing.B) {
	for _, pageSize := range []uint32{8, 1024} {
		for _, count := range []int{64, 1024, 8192} {
			b.Run(fmt.Sprintf("page%d/records%d", pageSize, count), func(b *testing.B) {
				slab, err := newDescriptorSlab(Config{PageSize: pageSize})
				if err != nil {
					b.Fatal(err)
				}
				b.Cleanup(slab.Close)
				refs := make([]SmallBytesRef, count)
				for i := range refs {
					slot, generation, err := slab.Allocate()
					if err != nil {
						b.Fatal(err)
					}
					if err := slab.Write(slot, vectorDescriptor{offset: uint32(i)}); err != nil {
						b.Fatal(err)
					}
					refs[i] = SmallBytesRef{slot, generation}
				}
				b.ReportAllocs()
				b.ResetTimer()
				random := uint32(9)
				for i := 0; i < b.N; i++ {
					random = random*1664525 + 1013904223
					index := random & uint32(count-1)
					ref := refs[index]
					record, live := slab.readLive(ref.Slot, ref.Generation)
					if !live || record.offset != index {
						b.Fatal(index, record, live)
					}
				}
			})
		}
	}
}
