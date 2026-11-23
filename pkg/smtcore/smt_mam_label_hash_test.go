package smtcore

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	uhash "github.com/glycerine/g3/pkg/util/hash"
)

func TestSMTMAMLabelHashSparseGrowth(t *testing.T) {
	var h smtMAMLabelHasher
	for _, id := range []uint32{3, 4096, 7, 3, 4096} {
		want := uint8(uhash.Mix(17, 3, id) & 63)
		if got := h.hash(id); got != want {
			t.Fatalf("declaration %d has label hash %d, want %d", id, got, want)
		}
	}
	var out strings.Builder
	h.display(&out)
	want := fmt.Sprintf("lbl-hasher:\n3 -> %d, 7 -> %d, 4096 -> %d\n",
		uhash.Mix(17, 3, 3)&63, uhash.Mix(17, 3, 7)&63, uhash.Mix(17, 3, 4096)&63)
	if out.String() != want {
		t.Fatalf("display after sparse growth %q, want %q", out.String(), want)
	}
}

func BenchmarkSMTMAMLabelHash(b *testing.B) {
	for _, count := range []uint32{64, 4096, 32768} {
		b.Run(strconv.Itoa(int(count)), func(b *testing.B) {
			b.ReportAllocs()
			var sum uint64
			for i := 0; i < b.N; i++ {
				var h smtMAMLabelHasher
				for id := uint32(0); id < count; id++ {
					sum += uint64(h.hash(id))
				}
			}
			if sum == 0 {
				b.Fatal("missing label hashes")
			}
		})
	}
}
