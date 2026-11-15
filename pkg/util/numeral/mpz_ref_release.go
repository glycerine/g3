//go:build !g3checked

package numeral

import (
	"github.com/glycerine/g3/pkg/arena"
)

type NumericCellRef uint32

func MakeNumericRef(slot arena.Slot, _ arena.Generation) NumericCellRef { return NumericCellRef(slot) }
func NumericSlot(ref NumericCellRef) arena.Slot                         { return arena.Slot(ref) }
func ValidNumericRef(s *numericCellSlab, ref NumericCellRef) bool {
	// Read succeeds exactly when the record is live (recordPtr non-nil);
	// test that without copying the record out.
	return s.recordPtr(NumericSlot(ref)) != nil
}
