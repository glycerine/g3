package api

import (
	"sync"
	"testing"
)

// Compare assignments of the same 419 verification conditions to 16 owners.
// Each goroutine keeps its solver and proof batch for its whole assignment.
func BenchmarkRaftWorkerPartitions(b *testing.B) {
	const workers = 16
	for _, width := range []string{"mixed", "onebit"} {
		queries := raftProofQueries(b, width)
		for _, policy := range []string{"Contiguous", "InputBytes", "Blocks1", "Blocks4", "Blocks16"} {
			b.Run(width+"/"+policy, func(b *testing.B) {
				parts := partitionRaftQueries(queries, workers, policy)
				var count int
				for _, part := range parts {
					if len(part) == 0 {
						b.Fatal("empty worker assignment")
					}
					count += len(part)
				}
				if count != len(queries) {
					b.Fatalf("assigned %d queries, want %d", count, len(queries))
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var group sync.WaitGroup
					var errs [workers]error
					group.Add(workers)
					for w := range parts {
						go func(w int) {
							defer group.Done()
							errs[w] = runRaftProofQueries(parts[w], true)
						}(w)
					}
					group.Wait()
					for w, err := range errs {
						if err != nil {
							b.Fatalf("worker %d: %v", w, err)
						}
					}
				}
				b.ReportMetric(float64(count), "obligations/op")
			})
		}
	}
}

func partitionRaftQueries(queries []string, workers int, policy string) [][]string {
	parts := make([][]string, workers)
	switch policy {
	case "Contiguous":
		for w := range parts {
			parts[w] = queries[w*len(queries)/workers : (w+1)*len(queries)/workers]
		}
	case "InputBytes":
		var total, assigned int64
		for _, q := range queries {
			total += int64(len(q))
		}
		w, start := 0, 0
		for i, q := range queries {
			assigned += int64(len(q))
			if w+1 < workers && (assigned >= total*int64(w+1)/int64(workers) || len(queries)-i-1 == workers-w-1) {
				parts[w] = queries[start : i+1]
				start = i + 1
				w++
			}
		}
		parts[w] = queries[start:]
	case "Blocks1", "Blocks4", "Blocks16":
		block := map[string]int{"Blocks1": 1, "Blocks4": 4, "Blocks16": 16}[policy]
		for i, q := range queries {
			w := i / block % workers
			parts[w] = append(parts[w], q)
		}
	default:
		panic("unknown Raft benchmark partition")
	}
	return parts
}
