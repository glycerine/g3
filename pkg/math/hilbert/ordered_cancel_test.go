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
	if calls != 2 {
		t.Fatalf("checkpoint calls = %d, want 2", calls)
	}
}
