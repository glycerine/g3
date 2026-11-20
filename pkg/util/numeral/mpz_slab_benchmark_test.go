package numeral

import (
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Compare the waste in a short-lived owner with the allocation and lookup
// costs of a large, densely populated numeric-cell slab.
func BenchmarkNumericCellPageSize(b *testing.B) {
	for _, live := range []int{8, 4096, 262144} {
		for _, size := range []uint32{0, 16, 32, 64, 128, 256, 1024} {
			name := strconv.Itoa(live) + "/" + strconv.Itoa(int(size))
			fill := func(b *testing.B) *numericCellSlab {
				s, err := newNumericCellSlab(arena.Config{PageSize: size})
				if err != nil {
					b.Fatal(err)
				}
				for i := 1; i <= live; i++ {
					slot, _, err := s.Allocate()
					if err != nil || int(slot) != i {
						b.Fatalf("allocate %d gave slot %d, err %v", i, slot, err)
					}
					if err := s.Write(slot, NumericCell{Size: uint32(i)}); err != nil {
						b.Fatal(err)
					}
				}
				return s
			}
			b.Run("Create/"+name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					s := fill(b)
					s.Close()
				}
			})
			b.Run("Read/"+name, func(b *testing.B) {
				s := fill(b)
				defer s.Close()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Odd stride visits all cells in a power-of-two working set.
					index := (uint32(i) * 40503) & uint32(live-1)
					slot := arena.Slot(index + 1)
					cell, err := s.Read(slot)
					if err != nil || cell.Size != uint32(slot) {
						b.Fatalf("read %d gave %d, err %v", slot, cell.Size, err)
					}
				}
			})
		}
	}
}
