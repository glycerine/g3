package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestManagerScalarFacadeFailureAndOwnership(t *testing.T) {
	local := newAnySturmFixture(t)
	foreign := newAnySturmFixture(t)
	before := local.alpha.refs
	if result, err := local.base.store.addValueRational(local.alpha, nil, true); !errors.Is(err, arena.ErrInvalidSlot) || result != nil {
		t.Fatal(result, err)
	}
	if result, err := local.base.store.divideValueInteger(local.alpha, nil, false); !errors.Is(err, arena.ErrInvalidSlot) || result != nil {
		t.Fatal(result, err)
	}
	for _, relation := range []func() (bool, error){
		func() (bool, error) {
			return local.base.store.notEqualAnyDeterminedValues(local.alpha, foreign.alpha, true)
		},
		func() (bool, error) {
			return local.base.store.lessEqualAnyDeterminedValues(local.alpha, foreign.alpha, true)
		},
		func() (bool, error) {
			return local.base.store.greaterEqualAnyDeterminedValues(local.alpha, foreign.alpha, true)
		},
		func() (bool, error) { return local.base.store.notEqualValueRational(local.alpha, nil) },
	} {
		answer, err := relation()
		if !errors.Is(err, arena.ErrInvalidSlot) || answer {
			t.Fatal(answer, err)
		}
	}
	if local.alpha.refs != before {
		t.Fatal("temporary owner leaked", before, local.alpha.refs)
	}
}
