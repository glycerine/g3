package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestBoundedRationalSignDeterminationAllocationPrefixes(t *testing.T) {
	failures, successes := 0, 0
	for prefix := int64(0); prefix < 1024; prefix++ {
		budget := arena.NewBudget(1 << 27)
		service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
			Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		bounds, err := numeral.NewBinaryRationalManager(service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: service})
		if err != nil {
			t.Fatal(err)
		}
		store := &realValueStore{numbers: service.Manager}
		one, err := store.newRational(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		minusOne, err := store.newRational(-1, 1)
		if err != nil {
			t.Fatal(err)
		}
		huge, err := store.newRational(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.numbers.SetString(&huge.rational, "184467440737095516170000000000000000000000000000000000000000000000000000000000000000000000001"); err != nil {
			t.Fatal(err)
		}
		interval, err := NewBinaryInterval(bounds)
		if err != nil {
			t.Fatal(err)
		}
		lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
		for _, setupErr := range []error{bounds.SetInt32(&lower, -2), bounds.SetInt32(&upper, 2), interval.SetFinite(&lower, &upper, false, false)} {
			if setupErr != nil {
				t.Fatal(setupErr)
			}
		}
		state := newSignDetermination(3)
		if err := state.attachInitialPolynomial(store, one); err != nil {
			t.Fatal(err)
		}
		p := []*realValue{nil, minusOne, nil, one}
		q := []*realValue{nil, huge}
		budget.FailAfter(prefix)
		kept, _, opErr := state.advanceBoundedRational(p, q, interval, true)
		budget.FailAfter(-1)
		if opErr != nil {
			failures++
		} else {
			successes++
			if !kept {
				t.Fatalf("prefix %d: successful discriminator was not kept", prefix)
			}
		}
		if err := store.cleanupPendingPolynomialSequence(); err != nil {
			t.Fatalf("prefix %d pending cleanup after %v: %v", prefix, opErr, err)
		}
		if err := state.closePolynomials(); err != nil {
			t.Fatalf("prefix %d state cleanup after %v: %v", prefix, opErr, err)
		}
		for _, cleanupErr := range []error{
			store.release(huge), store.release(one), store.release(minusOne), interval.Close(),
			bounds.Del(&lower), bounds.Del(&upper), bounds.CloseChecked(), service.Close(),
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
		t.Fatalf("allocation sweep missed failure or success: failures=%d successes=%d", failures, successes)
	}
}
