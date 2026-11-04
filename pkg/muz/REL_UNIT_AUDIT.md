# `tbv` source-unit audit

Scope: `src/muz/rel/tbv.h` and `src/muz/rel/tbv.cpp`, marked ported in `port_manifest.json` before this audit. This is a source and consumer audit, not a claim that all DOC, UDoc, or DDNF units are complete. The source search covered `src` for `tbv`, `tbv_manager`, `tbv_ref`, `tbv_vector` and `utbv`, including declarations and tests. The Go implementation is `pkg/muz/rel/tbv.go`.

## Live consumers and ownership

- `src/muz/rel/doc.h` and `doc.cpp` own a manager and its full vector, DOC positive/negative vectors, and temporary `tbv_ref` objects. They use copies, intersection, containment, concatenating slice copies, projection, permutation, complement and formulas. Go's concrete owners and operations are in `doc_cube.go`, `doc_projection.go`, `doc_union.go` and `doc_solver.go`. In particular `tbvUnion.insert` consumes inserted or subsumed vectors and `DocManager` closes its vector manager. The source `fixed_bit_vector::set` copies from bit zero of the source into the requested destination range; `SetSlice` does the same, including the second half of a join.
- `src/muz/rel/udoc_relation.h` and `udoc_relation.cpp` consume DOC/TBV for facts, selection, joins, projection and formula construction. The Go typed relation and formula path are in `udoc_relation_typed.go`, `udoc_relation_typed_operations.go` and `udoc_relation_typed_formula.go`. The relation's wider dispatch and lifecycle are separate audit obligations.
- `src/muz/ddnf/ddnf.h` and `ddnf.cpp` keep width-specific managers and vectors in their DAG and expression mapping, and use TBV allocation, intersection, hashing and lookup. Go's corresponding owners are `pkg/muz/ddnf/core.go`, `registry.go` and `preprocess.go`. Their engine's full behavior is a separate obligation.
- Source test consumers are `src/test/tbv.cpp`, `doc.cpp`, `udoc_relation.cpp` and `ddnf.cpp`; `src/test/main.cpp` registers tests. `src/util/fixed_bit_vector.h` provides the backing representation rather than another TBV consumer. No other production source consumer was found by the above search.

## Declaration and branch mapping

| Source surface | Go mapping and evidence |
| --- | --- |
| `tbit`, `neg`, vector indexing | `TBit`, `NegTBit`, `Get` and `Set`; encoding, intersection and union are exercised by `TestTBVSourceEncodingRangesAndSetOperations`. |
| Manager allocation, copies, integer/rational/range/string/permutation construction | `Allocate`, filled allocators, `AllocateCopy`, `AllocateUint64`, `AllocateBigInt`, `AllocateUint64Range`, `AllocateString`, `AllocatePermutation`; integer bit extraction uses `big.Int`, appropriate to the integral live numeral consumers. Parsing stops at the same first unsupported character and skips leading spaces/tabs. `TestTBVSourceParsingPermutationComplementAndProjection` covers the principal paths. |
| `reset`, deallocation, size and fill | `Reset`, `Deallocate`, `Close`, `SizeEstimateBytes`, `Fill`. Zero-width backing is shared in source; Go uses distinct wrappers and skips backing deallocation for zero width. Closed, foreign and stale wrappers are rejected. The live-wrapper ledger is removed on successful deallocation; see ownership verification below. |
| OR, AND, well-formedness, complement, equality, hash, whole and column containment, intersection | `SetOr`, `SetAnd`, `WellFormed`, `Complement`, `Equals`, `Hash`, `Contains`, `ContainsColumns`, `Intersect`; `tbv_cpp_test.go` ports the source driver over widths and `tbv_test.go` exercises the other operations. A failed Go complement returns the already-created slice to the caller, which owns those elements; callers must clean it on failure. |
| Range set from integer, rational or vector; projection | `SetUint64`, `SetBigInt`, `SetSlice`, `Project`. Projection reads through the source owner and allocates in the destination manager, which is necessary when widths differ. Bounds and allocation failures are Go error paths beyond source assertions. |
| Display and formula helpers | `DisplayRange`, `String`, `ToFormula`, `MkVar`; source displays highest bit first and `[]` for width zero, while parsing starts at bit zero. Formulas omit X and Z and use numeric Boolean constants. `TestTBVFormulaAndLifetimeBoundaries` and `TestTBVZeroWidthAndBounds` cover these cases. |
| `tbv_ref`, `tbv::eq`/`hash`, debug allocation hooks | Go uses explicit ownership and deferred cleanup in consumers rather than an RAII wrapper; equality and hash are manager methods. The source debug-only dangling diagnostics and global trace flag have no equivalent. No retained source consumer invokes `debug_alloc`. |

## Ownership verification

`TBVManager` tracks only currently live wrappers. Successful deallocation first releases the backing vector, then removes the wrapper in constant time by swapping the final ledger entry and clearing the vacated slot. A backing refusal leaves both wrapper and ledger intact for retry. Stale wrappers remain marked dead, and reset and close invalidate all remaining live wrappers. `TestTBVLiveLedgerBoundedAndInvalidated` covers 10,000 allocate/deallocate cycles with a persistent live vector, middle removal, both zero and nonzero widths, reset and close. `TestTBVDeallocateRefusalPreservesLedgerForRetry` covers foreign-manager rejection and simulated backing-reference refusal followed by a successful retry. The ledger retains capacity based on peak concurrent ownership, rather than cumulative allocations; reset and close release it.

## Verification

`TestTBVSourceSingleBitTruthTables` checks all four source bit states for conjunction, disjunction and both containment modes; `TestTBVSourceComplementOrderAndUnknownBits` checks order, undefined-bit handling and input preservation.

`cd pkg && go test ./muz/rel ./muz/ddnf -run 'TestTBV|TestDoc|TestDDNF' -count=1 -timeout=180s` passed (rel 0.011s, ddnf 0.047s). The same focused command was rerun after the ledger fix, including the new TBV tests. No full native suite was run for this audit.
