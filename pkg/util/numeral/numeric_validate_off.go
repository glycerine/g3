//go:build !g3checked && !g3debug

package numeral

// numericValidateCells selects whether digit accesses re-validate the cell's
// pooled memory object (SourceCellBacking.location: RawLocation and
// ObjectExtent) and copy the cell descriptor out of its slab on every word,
// as ast_validate_handles_* does for AST reads. z3 indexes mpz digits
// directly. Normal builds use the cached raw location and a pointer to the
// descriptor; the memory runtime still bounds-checks the raw block and checks
// initialized bytes. A digit access on a freed or recycled cell is a caller
// bug that only g3checked/g3debug builds report.
const numericValidateCells = false
