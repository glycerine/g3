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
complete short package suites after the remaining numeric and Karr work.

### `bound_relation` and `interval_relation`

Both C++ plugins accept signatures consisting entirely of integer or real
sorts ([dl_bound_relation.cpp](../src/muz/rel/dl_bound_relation.cpp#L32),
[dl_interval_relation.cpp](../src/muz/rel/dl_interval_relation.cpp#L37)). They
can be selected as the `check_relation` base for numeric original columns;
the trailing rule-proof column is assigned to the explanation relation by the
relation-level sieve. Go has separate bound/interval implementations and
product helpers, but the generic formula-checking explanation path currently
only constructs finite-table products. Add source-faithful checker dispatch
and test numeric relation-level explanations, proof-column separation,
formula conversion, and operation coverage.

The live Go rejection is explicit in `ensureBoundDatalog` and
`ensureIntervalDatalog` ([datalog_bound_dispatch.go](muz/fp/datalog_bound_dispatch.go),
[datalog_interval_dispatch.go](muz/fp/datalog_interval_dispatch.go)). The
required product path must carry the numeric relation through source joins,
projections, assignment filters, unions/deltas, and formula checks; translating
the numeric relation into a finite table would not preserve C++ behavior.

**Current worktree status:** relation-level numeric products now support
interval-backed joins, projection, numeric equality and interpreted filters,
union, and formula export. The relation-level compiler can construct numeric
full columns and numeric head constants. Datalog dispatch routes selected
relation-level numeric explanation queries through that compiler, and typed
zero-arity finite seeds are carried into numeric products. An end-to-end
interval test covers a numeric fact, an unbound head variable, recursive
compilation, and a ground expression query. The interval and fixedpoint package
short suites pass.

**Still open:** the current end-to-end coverage does not yet establish bound
relation behavior, checked numeric relation behavior, non-nullary seeds held in
the specialized activation stores, magic-set auxiliary products, or a broad
parity audit of every numeric compiler operation. Do not mark this item
complete until those paths are tested against the corresponding C++ behavior.

### `karr_relation`

The C++ plugin accepts signatures when Karr analysis is enabled in the
fixedpoint context ([karr_relation.h](../src/muz/rel/karr_relation.h#L48)).
Under that option it is therefore a valid `check_relation` base. Determine
whether the corresponding Go engine exposes this live configuration and Karr
relation operations to relation-level explanations; if it does, port the
checker/product route and test it. Otherwise document the specific missing
consumer/configuration gate rather than treating the plugin name alone as
unsupported.

Go exposes the same `xform.karr` configuration and relation implementation,
but `ensureKarrDatalog` currently rejects generated explanations with
`ErrKarrDatalogFragment` ([datalog_karr_dispatch.go](muz/fp/datalog_karr_dispatch.go)).
Thus this is a live-consumer parity gap, subject to the same generic relation
product/compiler work as the numeric relations.

**Current worktree status:** Karr uses the shared partial numeric product
adapter, but no end-to-end relation-level explanation query has been tested.
Seed transfer, Karr-specific filter/union behavior, formula validation, and
the `xform.karr` dispatch boundary remain unverified.

## Remaining implementation order

1. Audit and port `bound_relation` and `interval_relation` through relation
   explanation products and their compiler operations; validate numeric facts,
   recursive derivation, proof-column handling, and formula checks.
2. Audit and port `karr_relation` through the same pipeline when the Go engine's
   Karr context option is enabled; test both enabled and disabled selection.

All three need focused source-to-Go behavior tests. Do not close an item solely
because its constructor or unit-level storage tests pass: require an engine
query exercising the selected backing.
