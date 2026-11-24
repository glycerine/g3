package sat

import (
	"math/rand"
	"sort"
	"strconv"
	"testing"
)

var literalSortShapes = []string{
	"dense", "aligned-high", "cross-page", "sparse", "twice", "late-duplicate",
	"ascending", "descending", "mostly-ordered",
}

func literalSortInput(n int, shape string, seed int64) []Literal {
	rng := rand.New(rand.NewSource(seed))
	values := make([]Literal, n)
	for i, x := range rng.Perm(4096)[:n] {
		values[i] = Literal(x)
		switch shape {
		case "aligned-high":
			values[i] += 0xfffff000
		case "cross-page":
			values[i] += 4070
		case "sparse":
			values[i] = Literal(rng.Uint32())
		case "twice":
			if i%2 == 1 {
				values[i] = values[i-1]
			}
		case "late-duplicate":
			if i == n-1 {
				values[i] = values[0]
			}
		case "ascending":
			values[i] = Literal(i * 2)
		case "descending":
			values[i] = Literal(n - i)
		case "mostly-ordered":
			values[i] = Literal(i)
			if i > 2 && i%31 == 0 {
				values[i], values[i-2] = values[i-2], values[i]
			}
		}
	}
	return values
}

func TestLiteralSortPreservesEveryWord(t *testing.T) {
	var order satLiteralOrder
	for _, n := range []int{0, 1, 15, 16, 17, 31, 32, 64, 256, 1024, 4096} {
		for _, shape := range literalSortShapes {
			for seed := int64(0); seed < 4; seed++ {
				got := literalSortInput(n, shape, seed)
				want := append([]Literal(nil), got...)
				sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
				order.sort(got)
				for i, lit := range want {
					if got[i] != lit {
						t.Fatalf("size %d, %s, seed %d, word %d: got %d, want %d", n, shape, seed, i, got[i], lit)
					}
				}
			}
		}
	}
}

func BenchmarkLiteralSort(b *testing.B) {
	for _, n := range []int{16, 64, 256, 1024} {
		for _, shape := range literalSortShapes {
			b.Run(strconv.Itoa(n)+"/"+shape, func(b *testing.B) {
				var inputs [16][]Literal
				for i := range inputs {
					inputs[i] = literalSortInput(n, shape, int64(i+17))
				}
				var order satLiteralOrder
				values := make([]Literal, n)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					copy(values, inputs[i%len(inputs)])
					order.sort(values)
				}
			})
		}
	}
}
