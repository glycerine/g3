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

### `karr_relation`

The C++ plugin accepts signatures when Karr analysis is enabled in the
fixedpoint context ([karr_relation.h](../src/muz/rel/karr_relation.h#L48)).
Under that option it is therefore a valid `check_relation` base. Determine
whether the corresponding Go engine exposes this live configuration and Karr
relation operations to relation-level explanations; if it does, port the
checker/product route and test it. Otherwise document the specific missing
consumer/configuration gate rather than treating the plugin name alone as
unsupported.
