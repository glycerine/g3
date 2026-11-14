//go:build !g3checked && !g3debug

package numeral

// numericValidateCells selects whether digit accesses re-validate the cell's
// pooled memory object (SourceCellBacking.location: RawLocation and
// ObjectExtent) and copy the cell descriptor out of its slab on every word,
// as ast_validate_handles_* does for AST reads. z3 indexes mpz digits
// directly. Digits live in the cell object's native word payload
// (SmallAllocator.AttachWords); normal builds index that slice through a
// pointer to the descriptor, with only a bounds check, and write the size
// header through the cached raw location. A digit access on a freed or
// recycled cell is a caller bug that only g3checked/g3debug builds report.
const numericValidateCells = false
