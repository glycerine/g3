package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRealClosureBoundedRationalSignCountProducer(t *testing.T) {
	budget := arena.NewBudget(1 << 25)
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
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	one, err := store.newRational(1, 1)
	must(err)
	minusOne, err := store.newRational(-1, 1)
	must(err)
	p := []*realValue{minusOne, nil, one} // x^2 - 1
	interval, err := NewBinaryInterval(bounds)
	must(err)
	lower, upper := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	must(bounds.SetInt32(&lower, -2))
	must(bounds.SetInt32(&upper, 2))
	must(interval.SetFinite(&lower, &upper, false, false))
	for _, usePrem := range []bool{true, false} {
		for _, tc := range []struct {
			name                      string
			q                         []*realValue
			equal, positive, negative int
			queried                   bool
		}{
			{"mixed", []*realValue{nil, one}, 0, 1, 1, true},
			{"one-zero-negative", []*realValue{minusOne, one}, 1, 0, 1, false},
			// The first source branch proves positivity, but the independent
			// second if/else still takes the q^2 path.
			{"all-positive-source-quirk", []*realValue{one}, 0, 2, 0, true},
		} {
			equal, positive, negative, queried, err := store.countSignsAtZerosRational(p, tc.q, interval, 2, usePrem)
			if err != nil || equal != tc.equal || positive != tc.positive || negative != tc.negative || queried != tc.queried {
				t.Fatalf("%s use_prem=%v: counts=(%d,%d,%d), queried=%v, err=%v", tc.name, usePrem, equal, positive, negative, queried, err)
			}
		}
	}
	must(store.release(one))
	must(store.release(minusOne))
	must(interval.Close())
	must(bounds.Del(&lower))
	must(bounds.Del(&upper))
	must(bounds.CloseChecked())
	must(service.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("bounded sign-count producer retained bytes", used)
	}
}

func TestRealClosureBoundedSignCountAllocationPrefixes(t *testing.T) {
	failures, successes := 0, 0
	for prefix := int64(0); prefix < 512; prefix++ {
		budget := arena.NewBudget(1 << 25)
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
		p := []*realValue{minusOne, nil, one}
		q := []*realValue{nil, huge}
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
		budget.FailAfter(prefix)
		_, _, _, queried, opErr := store.countSignsAtZerosRational(p, q, interval, 2, true)
		budget.FailAfter(-1)
		if opErr != nil {
			failures++
		} else {
			successes++
			if !queried {
				t.Fatalf("prefix %d: mixed signs skipped q^2", prefix)
			}
		}
		for _, cleanupErr := range []error{
			store.cleanupPendingPolynomialSequence(), store.release(huge), store.release(one), store.release(minusOne),
			interval.Close(), bounds.Del(&lower), bounds.Del(&upper), bounds.CloseChecked(), service.Close(),
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
