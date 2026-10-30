package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestParallelUnitExchangeOwnershipAndClose(t *testing.T) {
	p := NewParallelUnitExchange()
	forged := &ParallelUnitExchange{self: p}
	if _, _, err := forged.Exchange(nil, 0); !errors.Is(err, ErrSATOwner) {
		t.Fatal(err)
	}
	satOK(t, p.Close())
	if _, err := p.Len(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
	satOK(t, p.Close())
}
