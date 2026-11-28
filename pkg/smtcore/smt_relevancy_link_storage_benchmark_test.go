package smtcore

import (
	"strconv"
	"testing"
)

// Measure registration storage for a fresh solver when one expression gains
// many handlers. The separately timed watch-pop benchmark reuses storage.
func BenchmarkSMTRelevancyLinkStorage(b *testing.B) {
	m := astTestManager(b)
	owner := smtRelevancyBool(b, m, "watch-owner")
	astTestPin(b, m, owner)
	for _, count := range []int{1, 32, 256, 4096} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var table smtRelevancyTable
				for j := 0; j < count; j++ {
					table.push(owner, uint32(j+1))
				}
				for j := count; j > 0; j-- {
					if table.pop(owner) != uint32(j) {
						b.Fatal("registration order")
					}
				}
			}
		})
	}
}
