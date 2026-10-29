package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRationalSquareFreeAllocationPrefixes(t *testing.T) {
	failures, successes := 0, 0
	for prefix := int64(0); prefix < 4096; prefix++ {
		budget := arena.NewBudget(1 << 27)
		service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
			Cells: arena.Config{PageSize: 1, Budget: budget}, Words: arena.VectorConfig{PageSize: 1, Descriptors: arena.Config{PageSize: 1, Budget: budget}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		store := &realValueStore{numbers: service.Manager}
		coefficients := make([]*realValue, 4)
		for i, text := range []string{
			"368934881474191032340000000000000000000000000000000000000000000000000000000000000000000000002",
			"-553402322211286548510000000000000000000000000000000000000000000000000000000000000000000000003",
			"0",
			"184467440737095516170000000000000000000000000000000000000000000000000000000000000000000000001",
		} {
			if text == "0" {
				continue
			}
			coefficients[i], err = store.newRational(1, 1)
			if err == nil {
				err = store.numbers.SetString(&coefficients[i].rational, text)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		budget.FailAfter(prefix)
		result, opErr := store.squareFreeRationalPolynomial(coefficients, true)
		budget.FailAfter(-1)
		if opErr != nil {
			failures++
		} else {
			successes++
			if len(result) != 3 {
				t.Fatalf("prefix %d successful degree=%d", prefix, len(result)-1)
			}
		}
		if err := store.releaseSturmCoefficients(result); err != nil {
			t.Fatalf("prefix %d result cleanup after %v: %v", prefix, opErr, err)
		}
		if err := store.cleanupPendingPolynomialSequence(); err != nil {
			t.Fatalf("prefix %d pending cleanup after %v: %v", prefix, opErr, err)
		}
		for _, coefficient := range coefficients {
			if err := store.release(coefficient); err != nil {
				t.Fatalf("prefix %d coefficient cleanup after %v: %v", prefix, opErr, err)
			}
		}
		if err := service.Close(); err != nil {
			t.Fatalf("prefix %d service cleanup after %v: %v", prefix, opErr, err)
		}
		if used := budget.Used(); used != 0 {
			t.Fatalf("prefix %d retained %d bytes after %v", prefix, used, opErr)
		}
		if opErr == nil {
			break
		}
	}
	if failures == 0 || successes == 0 {
		t.Fatalf("allocation sweep failures=%d successes=%d", failures, successes)
	}
}
