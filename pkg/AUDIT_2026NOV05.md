# Port audit — 2026-11-05

This audit records additional port work found by comparing current live Go
execution paths with the checked-in C++ source after the fixedpoint completion
record was sealed. It does not replace or modify `muz/FIXED_POINT_FINISH.md` or
`FILES_LEFT.md`.

## Confirmed additional work

### Relation-level explanation checking must dispatch through the selected `check_relation` backing

**Go gap.** `PropositionalEngine.beginDatalogExplanationFormulaScope` accepts
only `hashtable` and nested `check`; it returns `ErrCompilerUnsupported` for
`sparse` and `lazy_sparse` ([datalog_explanation_formula_scope.go](muz/fp/datalog_explanation_formula_scope.go:22)). The corresponding test explicitly
asserts that Sparse is unavailable ([datalog_explanation_formula_scope_test.go](muz/fp/datalog_explanation_formula_scope_test.go:77)). This prevents
relation-level explanation queries from reaching formula checking with those
selected finite backends. The parameter parser already recognizes these plugin
names, and Go has concrete Sparse and LazySparse finite-table implementations.

**C++ behavior.** `rel_context::updt_params` installs `check_relation` as the
favourite relation plugin and assigns its base to the plugin named by
`datalog.check_relation` ([rel_context.cpp](muz/rel/cppsrc/rel/rel_context.cpp:589)).
The wrapper's `can_handle_signature` delegates to that base plugin, rather
than restricting by plugin name ([check_relation.cpp](muz/rel/cppsrc/rel/check_relation.cpp:167));
its operation factories forward to the chosen base and verify the resulting
relation formula. Relation-level explanation construction selects the inner
sieve kind through the relation manager and unions into the selected relation
([dl_mk_explanations.cpp](muz/rel/cppsrc/rel/dl_mk_explanations.cpp:637),
[dl_mk_explanations.cpp](muz/rel/cppsrc/rel/dl_mk_explanations.cpp:783)). For a
signature accepted by Sparse, this source path does not reject the query just
because the backing name is `sparse`.

**Status: implemented for Sparse and LazySparse.** The Go explanation scope now
dispatches through `sparse` and `lazy_sparse`, preserving the selected primary
inside formula-checked explanation products. It keeps explicit per-predicate
representation requests outside the favourite checker and falls back to Hash
for nullary signatures that the selected sparse plugin declines. The new
constructor test checks the concrete primary and formula-check state for both
backings; the expression explanation test exercises both selections end to
end. Existing `FormulaCheckedFinitePair` lifecycle tests cover clone, join,
projection, selection, union/delta, formula validation, and ownership for the
selected finite backing wrappers.

**Verification.** Passed focused tests for formula-scope dispatch, fresh
Sparse/LazySparse products, and end-to-end relation-level explanations. Also
passed `go test -short ./pkg/muz/fp ./pkg/muz/rel -count=1 -timeout=0` after the
implementation; the added constructor test passed in the subsequent focused
run. Rerun both package suites after the remaining audit work is implemented.

**Implementation note.** The Go formula-checked finite wrapper currently
represents the selected Sparse/LazySparse table as the primary side of a
`FiniteCheckedPair`, with Hash as a consistency side, and attaches the formula
checker to that pair. This preserves the selected concrete primary and checks
its formula, but adds an internal concrete cross-check that C++
`check_relation` does not itself require. Keep this distinction visible during
future parity review; the C++ source evidence establishes selected backing
dispatch, not this extra Hash cross-check.

## Further confirmed relation-level checker combinations

The following are accepted by Go's `datalog.check_relation` parameter parser,
are selectable as the C++ `check_relation` base, and are not implemented by
the current relation-level Go explanation formula scope. They are additional
port work, not source-side unsupported combinations.

### `doc` / UDoc

The C++ `udoc_plugin::can_handle_signature` accepts only finite-valued sorts
([udoc_relation.cpp](../src/muz/rel/udoc_relation.cpp#L301)). The explanation
product's source/metadata sieves therefore use UDoc only for finite original
columns and keep the trailing rule-proof column in the explanation plugin.
Go has UDoc relation and formula-checking support, but the generic
`RelationExplanationProduct` construction currently assumes a `FiniteTable`.
Port relation-level explanation construction through UDoc while preserving
finite-signature capability checks, typed formula export, and the existing
Hash fallback for signatures UDoc declines.

The live Go rejection is explicit: `ensureUDocDatalog` returns
`ErrUDocDatalogFragment` whenever `datalog.generate_explanations` is enabled
([datalog_udoc_dispatch.go](muz/fp/datalog_udoc_dispatch.go)). UDoc stores
subsumed bit-vector cubes, so implementing this requires carrying UDoc's typed
relation operations through the relation-level product/compiler; converting
the cubes to enumerated finite tuples would change the selected representation
and its semantics.

**Prerequisite fixed:** `UDocSieve` previously rejected a formula-checked UDoc
factory during signature selection and adoption, which prevented the C++
`check_relation(doc)` wrapper from occupying the ordinary explanation sieve.
Go now accepts the checked inner relation, and the regression test verifies
checked full-sieve construction, membership, and clone. This makes the selected
UDoc checker composable with the existing UDoc product; it does not yet remove
the Datalog explanation rejection or add UDoc-backed explanation registers.

**Status: completed for the audited relation-level explanation path.**
`RelationExplanationProduct` has a UDoc
product backing for construction, clone, join, projection, selection, union,
formula export, and formula-check inspection. The explanation transformer now
transfers typed UDoc seed relations by typed union, without expanding their
cubes. Engine dispatch supports both `datalog.default_relation=doc` and
`datalog.check_relation=doc` for relation-level explanations. Query-clone
factory ownership, the nullary Hash fallback identity, interpreted filtering,
and DOC-to-DOC negation are covered by focused tests. Both mixed-join operand
orders are covered at the product layer, including selected-table-kind
preservation when the manager's current default differs. The engine regression
joins two DOC-backed BV predicates with a finite fallback Bool predicate,
projects the result, and checks the query proof in checked and unchecked modes.
Source `sieve_relation_plugin::mk_filter_by_negation_fn` asks the manager to
construct a filter over the two inner relations; when only one side is DOC,
both DOC and finite plugins decline that mixed pair, so no operation exists.
Go explicitly rejects that unsupported combination and tests that both inputs
remain unchanged. The specialized UDoc evaluator still declines mixed programs;
relation-level explanation queries with DOC selected use the generic compiler,
which applies per-signature DOC capability and the same ordinary fallback as
the source manager.

**Verification.** Passed `go test ./pkg/muz/rel -run 'UDoc|RelationExplanationProduct' -count=1 -timeout=0`
and `go test ./pkg/muz/fp -run 'DatalogDoc' -count=1 -timeout=0`. Rerun both
complete short package suites after the remaining numeric work.

### `bound_relation` and `interval_relation`

Both C++ plugins accept signatures consisting entirely of integer or real
sorts ([dl_bound_relation.cpp](../src/muz/rel/dl_bound_relation.cpp#L32),
[dl_interval_relation.cpp](../src/muz/rel/dl_interval_relation.cpp#L37)). They
can be selected as the `check_relation` base for numeric original columns;
the trailing rule-proof column is assigned to the explanation relation by the
relation-level sieve.

The audit initially identified a finite-only formula-checking explanation
path. Commit `93370e9d3` added a shared numeric product adapter, interval
relation-level query dispatch, numeric compiler columns/constants, and one
recursive interval explanation test. This is partial integration evidence,
not proof that the Go relation algorithms or all dispatch paths match C++.
Treat both Go implementations as candidates for a source-to-source behavioral
re-audit and replace code wherever that audit finds a divergence.

**Status: open; re-audit and rework the Go implementations against C++.** Use
the following sequence and record source locations and observed behavior for
each operation before changing its Go counterpart.

1. **Build a source behavior matrix.** Read the complete C++ plugin and
   relation implementation for each operation: signature capability, empty
   and full constructors, fact insertion, clone/reset, equality and identical
   filters, interpreted filters, join, projection, rename, union, delta, and
   widening. Include nullary relations, integer and real sorts, malformed
   inputs, unsupported operators, and each source quirk. Compare each entry
   with the existing Go implementation and its tests; label it matched,
   divergent, or not covered. Do not infer parity from matching method names.
2. **Rework `BoundRelation`.** Match C++'s equivalence classes and strict and
   weak order constraints, including their normalization and transitive
   closure. Verify empty/full and fact semantics; joins across shared columns;
   projection and rename; equality-class merging; strict/weak union behavior,
   delta calculation, and widening; intentionally coarse `filter_equal`; and
   the exact accepted grammar and effects of interpreted filters. Preserve
   source-visible no-ops and errors rather than improving the abstraction.
3. **Rework `IntervalRelation`.** Match C++ interval endpoints, open/closed
   bounds, infinities, empty/full state, numeral fact insertion, equality and
   identical filters, supported interpreted comparisons, joins, projection,
   rename, union, delta, and widening. Check real and integer boundary cases
   separately, including singleton intervals and contradictions.
4. **Match plugin dispatch and cross-plugin operations.** Verify which concrete
   operations each plugin constructs or declines, including bound union from
   interval sources and every product/plugin combination used by the Datalog
   engine. Audit `check_relation` wrapping to confirm it checks the selected
   backing's formula while keeping the trailing proof column in the
   explanation relation. Keep signature fallback behavior identical to the
   C++ relation manager.
5. **Re-audit ingestion and lifecycle.** Trace how C++ and Go move direct
   numeric facts, rule facts, and updates into relation seeds; how seeds are
   cloned into query contexts; and when they are reset or restored. Cover
   query clones, repeated queries, scopes, updates, failure cleanup, and
   formula-checker setup/teardown. Do not substitute finite tuple enumeration
   for a numeric relation or silently discard an existing typed seed.
6. **Establish operation-level tests.** For each matrix entry, add or correct
   a Go unit test that asserts the C++ result, including error/no-op behavior
   and state after failure. For checked operations, compare the relation
   formula to an independently constructed expected formula. Keep tests for
   both plugins and cross-plugin dispatch; do not remove existing coverage.
7. **Establish end-to-end tests.** Exercise default selection and
   `check_relation` selection for both plugins, proof-column separation,
   numeric facts and constants, positive recursive derivation, interpreted
   arithmetic guards, expression queries, seed updates, and supported
   magic-set transformations. Assert both answer status and exported proof
   formula. Include expected unsupported cases where C++ declines an
   operation.
8. **Close only on demonstrated parity.** Run focused source-matrix tests after
   each implementation change, then the full short `pkg/muz/rel` and
   `pkg/muz/fp` suites. Keep the item open until every matrix entry is matched
   or explicitly proven unreachable in retained Go consumers, and all
   required end-to-end tests pass.

The current interval explanation regression and short package runs are useful
baseline coverage, but do not replace these checks. Bound behavior, checked
numeric query integration, non-nullary typed activation seeds, and magic-set
auxiliary products are specifically unverified.

**Parity audit progress.** The first concrete divergence was in bound-relation
projection. C++ `bound_relation_helper::mk_project_t` remaps every stored edge
through the projection vector, including mapping references to removed columns
to `UINT_MAX`; Go had dropped those edges. Go now preserves that sentinel and
`TestBoundRelationJoinAndProjection` asserts it. The follow-on union behavior is
also source-specific: `bound_relation::mk_eq` copies edges by iterating only
valid source columns, so it drops the sentinel; Go now does the same, covered by
`TestBoundRelationUnionDropsProjectionSentinelLikeSourceMkEq`. Verified with
those focused tests and `go test ./pkg/muz/rel -short -count=1 -timeout=0`.
Empty-source projection and rename behavior is now confirmed at the relation
layer. C++ `vector_relation::mk_project` and `mk_rename` initialize a full
destination, copy the reset payload, and do not propagate `m_empty`; Go now
matches this for both plugins. A native C++ `dl_relation` regression directly
confirmed that empty interval projection and rename produce full relations,
and the Go relation suite passes with corresponding operation tests. The
behavior also changes Go fixedpoint query results where an empty intermediate
is projected: an initial fixedpoint package run exposed 29 failures, chiefly
status and empty-state expectations. Focused tests now also cover the resulting
nullary projection, ground-query status, contradictory selection/guard
operation order, clone cleanup, and disjoint shared-tail projection behavior.
The consequences are now traced through numeric/external and sparse/interval
products as well. Tests distinguish an empty join that becomes full after the
interval projection from a later contradictory constant filter that empties
the projected relation again. They assert continued guard/filter callbacks,
head singleton assembly, rename and union ordering, and the resulting
`l_false`/`l_undef` outcomes while retaining independent answer-formula,
source-state, and ownership checks. The affected relation tests pass with
`go test ./pkg/muz/rel -short -count=1 -timeout=0`; the relevant fixedpoint
suite passes with
`go test ./pkg/muz/fp -run '(BoundInterval|ExternalInterval|SparseInterval|IntervalDatalog)' -short -count=1 -timeout=0`.
This resolves the empty-source transform divergence and the integration cases
found from it, not the full numeric relation audit. Bound/interval fact
semantics, constraint normalization, join boundary cases, union/delta/
widening, interpreted-filter grammar, plugin dispatch combinations, and
failure-state parity still need operation-by-operation C++ comparison.
Nullary signatures are now directly covered for both plugins: full/empty tuple
membership, adding the empty tuple as a fact, full and empty joins, and the
source full-destination result when projecting an empty relation to arity zero
pass in `TestNumericRelationNullarySourceBehavior`.
The bound-from-interval union boundary is also pinned down: C++ compares only
endpoint values, dropping a strict edge when `upper(lhs) >= lower(rhs)` and a
weak edge only when `upper(lhs) > lower(rhs)`, without using endpoint openness.
`TestBoundRelationUnionIntervalIgnoresEndpointOpenness` covers equal open and
closed endpoints for both edge kinds.
The fact-membership audit found and fixed another direct divergence. C++
`interval_relation::contains_fact` checks equality classes and each interval
payload but does not inspect `m_empty`; an empty relation whose reset payload
is unbounded can therefore report a numeric tuple as contained. Go's
rational-tuple `ContainsFact` used to reject all empty relations before reading
that payload. It now matches the source, with unary and nullary regression
coverage alongside the existing AST-valued membership test.
The same payload distinction is now asserted for formula export: both plugins
produce `true` from an empty relation's reset, unconstrained columns, matching
the C++ `to_formula` methods that do not branch on `m_empty`.
The tests now also cover filtering an already-empty relation: the empty bit
stays set while `AddLess` or an interpreted interval equality updates the
backing payload, and formula export reflects that payload. This matches the
C++ mutators, which do not short-circuit on `empty()`.
Union/delta coverage now includes equality-class splitting for both relations:
the target and delta must both adopt the source-common partition, and a stable
repeat must leave the published delta unchanged. The test also confirms that
the copied interval delta has the source vector-copy behavior (fresh independent
columns with the copied interval payload).
Interval endpoint coverage now also exercises `IntervalRelation.Union` itself:
the ordinary union hull closes equal endpoints from either source, while
widening the same open/closed pair expands both endpoints to infinity, as in
the C++ plugin's asymmetric tie conditions.
Bound-constraint closure now follows another exact source quirk: its C++ walk
normalizes the initial left/right columns, but reads each subsequently visited
edge index directly from the backing vector without `find()`. Because equality
merges leave old nonrepresentative payloads in place, this can miss a strict
cycle through an equality alias and can omit bounds stored only on the current
representative. Go previously normalized every visited index and derived those
extra bounds. The Go walk now keeps the raw edge index, with regressions for both
the omitted derived bound and the missed cycle; the graph-closure oracle still
covers strict-cycle behavior without equality aliases.

**Operation matrix (partial).** “Matched” means the implementation and a
focused Go test currently agree with the read C++ branch; it does not imply
the full operation has exhaustive edge-case coverage.

| Operation | Bound relation | Interval relation | Evidence or open case |
| --- | --- | --- | --- |
| Signature capability | Matched | Matched | Both accept nullary and mixed Int/Real signatures; reject other sorts. |
| Empty/full construction and clone | Matched | Matched | Constructor, full-column, and copy/equality-class tests. |
| Equality and identical filters | Matched | Matched | Coarse bound `filter_equal`, interval point intersection, and n-ary merge tests. |
| Interpreted filters | Matched for tested grammar | Matched for tested grammar | Bound variable order/equality forms; interval real/Int strictness and unsupported conjunctions. |
| Join | Matched | Matched | Right bound indices follow the C++ unoffset-copy quirk; both plugins propagate empty operands. |
| Projection | Sentinel mapping and empty-source behavior matched | Empty-source behavior matched | Bound removed-column edges map to `UINT_MAX`; C++ generic transform drops the empty flag. Native C++ interval regression and Go relation tests cover empty source; fixedpoint product tests cover resulting filter/guard/head-assembly order. |
| Rename | Empty-source behavior matched | Empty-source behavior matched | C++ generic transform copies reset payload and does not propagate the empty flag; Go relation tests and affected fixedpoint tests cover the behavior. |
| Union/delta/widen | Matched for covered cases | Matched for covered cases | Bound union from interval ignores endpoint openness exactly as C++; interval tests cover hull, widening, delta state, and pruning. Remaining boundary matrix is incomplete. |
| Plugin/cross-plugin dispatch | Partial | Partial | Bound-from-interval union covered; explanation products and all consumer combinations remain open. |

The cited implementation branches are in
[`dl_vector_relation.h`](../src/muz/rel/dl_vector_relation.h#L178),
[`dl_bound_relation.cpp`](../src/muz/rel/dl_bound_relation.cpp#L32), and
[`dl_interval_relation.cpp`](../src/muz/rel/dl_interval_relation.cpp#L37).

### `karr_relation`

The Karr port is considered complete based on the existing Go implementation
and its tests matching the C++ Karr behavior. The C++ plugin can participate in
the generic relation-level explanation machinery when Karr is enabled, but
that composition does not add a separate requirement to this audit. Do not add
Karr-specific explanation support or tests unless a concrete divergence from
C++ Karr behavior is found.

## Remaining implementation order

1. Audit and port `bound_relation` and `interval_relation` through relation
   explanation products and their compiler operations; validate numeric facts,
   recursive derivation, proof-column handling, and formula checks.

Require focused source-to-Go behavior tests for the remaining numeric item;
do not close it solely because constructor or unit-level storage tests pass.
