package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestFastShellReplacesOnlyNumeralManagerExceptions(t *testing.T) {
	for _, sourceErr := range []error{numeral.ErrFastFloatDomain, numeral.ErrFastFloatOverflow} {
		got := sourceFastFailure(sourceErr)
		if !errors.Is(got, ErrSubpavingFailure) || errors.Is(got, numeral.ErrFastFloatDomain) {
			t.Fatalf("source MPFF exception translation for %v: %v", sourceErr, got)
		}
	}
	allocationErr := arena.ErrBudget
	if got := sourceFastFailure(allocationErr); got != allocationErr {
		t.Fatalf("source MPFF catch must preserve non-numeral errors: got %v, want %v", got, allocationErr)
	}
}
