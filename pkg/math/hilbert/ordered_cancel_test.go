package hilbert

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestOrderedSaturationCancellationStopsBeforeResolution(t *testing.T) {
	calls := 0
	b := New(func() bool { calls++; return calls == 1 })
	if err := b.AddEQ(rationals(1, -1), rational(0)); err != nil {
		t.Fatal(err)
	}
	result, err := b.Saturate()
	if err != nil || result != lbool.Undef {
		t.Fatalf("Saturate = %v, %v", result, err)
	}
	// saturate(): checkpoint 1 passes; saturate(ineq): the while test's
	// checkpoint 2 fails and the following if (!checkpoint()) is call 3.
	if calls != 3 {
		t.Fatalf("checkpoint calls = %d, want 3", calls)
	}
}
