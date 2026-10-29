package realclosure

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestRetainedDecimalValueRejectsForeignAndClosedValues(t *testing.T) {
	f := newRankedValueFixture(t, true)
	foreign := newRankedValueFixture(t, true)
	var out bytes.Buffer
	if err := f.base.store.displayValueDecimal(&out, foreign.alpha, f.alpha.root.owner.intervals, 8); !errors.Is(err, arena.ErrInvalidSlot) || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
	value, err := f.base.store.newRational(7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.base.store.release(value); err != nil {
		t.Fatal(err)
	}
	if err := f.base.store.displayValueDecimal(&out, value, f.alpha.root.owner.intervals, 8); !errors.Is(err, arena.ErrInvalidSlot) || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
}
