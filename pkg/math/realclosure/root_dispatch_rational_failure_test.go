package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRationalRootDispatchAllocationPrefixes(t *testing.T) {
	failures, successes, publishedPrefixes := 0, 0, 0
	for prefix := int64(0); prefix < 4096; prefix++ {
		budget := arena.NewBudget(1 << 27)
		service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
			Cells: arena.Config{PageSize: 1, Budget: budget}, Words: arena.VectorConfig{PageSize: 1, Descriptors: arena.Config{PageSize: 1, Budget: budget}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		bounds, err := numeral.NewBinaryRationalManager(service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: service})
		if err != nil {
			t.Fatal(err)
		}
		store := &realValueStore{numbers: service.Manager}
		minusTwo, err := store.newRational(-2, 1)
		if err != nil {
			t.Fatal(err)
		}
		one, err := store.newRational(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		huge := "184467440737095516170000000000000000000000000000000000000000000000000000000000000000000000001"
		if err = store.numbers.SetString(&one.rational, huge); err != nil {
			t.Fatal(err)
		}
		if err = store.numbers.SetString(&minusTwo.rational, "-368934881474191032340000000000000000000000000000000000000000000000000000000000000000000000002"); err != nil {
			t.Fatal(err)
		}
		owner := &algebraicRootOwner{values: store, intervals: bounds}
		budget.FailAfter(prefix)
		roots, opErr := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, true)
		budget.FailAfter(-1)
		if opErr != nil {
			failures++
			if len(roots) > 0 {
				publishedPrefixes++
			}
		} else {
			successes++
			if len(roots) != 2 {
				t.Fatalf("prefix %d successful result has %d roots", prefix, len(roots))
			}
		}
		for _, root := range roots {
			if err := store.release(root); err != nil {
				t.Fatalf("prefix %d root cleanup after %v: %v", prefix, opErr, err)
			}
		}
		if err := store.cleanupPendingPolynomialSequence(); err != nil {
			t.Fatalf("prefix %d pending cleanup after %v: %v", prefix, opErr, err)
		}
		for _, cleanupErr := range []error{
			store.release(minusTwo), store.release(one),
			bounds.CloseChecked(), service.Close(),
		} {
			if cleanupErr != nil {
				t.Fatalf("prefix %d cleanup after %v: %v", prefix, opErr, cleanupErr)
			}
		}
		if used := budget.Used(); used != 0 {
			t.Fatalf("prefix %d retained %d bytes after %v", prefix, used, opErr)
		}
		if opErr == nil {
			break
		}
	}
	if failures == 0 || successes == 0 {
		t.Fatalf("allocation sweep missed failure or success: failures=%d successes=%d prefixes=%d", failures, successes, publishedPrefixes)
	}
}
