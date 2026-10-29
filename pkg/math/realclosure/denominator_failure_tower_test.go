package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestTowerDenominatorFailurePreservesInputs(t *testing.T) {
	f := newTowerDenominatorFixture(t, true)
	before := f.gamma.refs
	if normalized, denominator, err := f.base.store.cleanTowerPolynomialDenominators(nil, true); !errors.Is(err, arena.ErrInvalidSlot) || normalized != nil || denominator != nil {
		t.Fatal(normalized, denominator, err)
	}
	if f.gamma.refs != before || f.gamma.deleted {
		t.Fatal("failure consumed input", before, f.gamma.refs)
	}
}
