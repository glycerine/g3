package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRationalSignIsolationAllocationPrefixes(t *testing.T) {
	failures, successes, publishedPrefixes := 0, 0, 0
	for prefix := int64(0); prefix < 2048; prefix++ {
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
		huge, err := store.newRational(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		text := "184467440737095516170000000000000000000000000000000000000000000000000000000000000000000000001"
		if err = store.numbers.SetString(&huge.rational, text); err != nil {
			t.Fatal(err)
		}
		scaled := make([]*realValue, 4)
		for i, n := range []int64{-6, 11, -6, 1} {
			scalar, scalarErr := store.newRational(n, 1)
			if scalarErr != nil {
				t.Fatal(scalarErr)
			}
			scaled[i], scalarErr = store.multiplyRationals(huge, scalar)
			if releaseErr := store.release(scalar); scalarErr == nil {
				scalarErr = releaseErr
			}
			if scalarErr != nil {
				t.Fatal(scalarErr)
			}
		}
		interval, err := NewBinaryInterval(bounds)
		if err != nil {
			t.Fatal(err)
		}
		lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
		endpoint := "1606938044258990275541962092341162602522202993782792835301376" // 2^200
		for _, setupErr := range []error{bounds.Z.SetString(&lower.Num, "-"+endpoint), bounds.Z.SetString(&upper.Num, endpoint), interval.SetFinite(&lower, &upper, false, false)} {
			if setupErr != nil {
				t.Fatal(setupErr)
			}
		}
		owner := &algebraicRootOwner{values: store, intervals: bounds}
		p := scaled // huge*(x-1)(x-2)(x-3)
		budget.FailAfter(prefix)
		values, opErr := owner.isolateRationalRootsBySigns(p, 3, interval, interval, true)
		budget.FailAfter(-1)
		if opErr != nil {
			failures++
			if len(values) != 0 {
				publishedPrefixes++
			}
		} else {
			successes++
			if len(values) != 3 {
				t.Fatalf("prefix %d: successful publication has %d roots", prefix, len(values))
			}
		}
		for _, value := range values {
			if err := store.release(value); err != nil {
				t.Fatalf("prefix %d published root cleanup after %v: %v", prefix, opErr, err)
			}
		}
		if err := store.cleanupPendingPolynomialSequence(); err != nil {
			t.Fatalf("prefix %d pending cleanup after %v: %v", prefix, opErr, err)
		}
		cleanup := []error{store.release(huge)}
		for _, coefficient := range scaled {
			cleanup = append(cleanup, store.release(coefficient))
		}
		cleanup = append(cleanup, interval.Close(), bounds.Del(&lower), bounds.Del(&upper), bounds.CloseChecked(), service.Close())
		for _, cleanupErr := range cleanup {
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
