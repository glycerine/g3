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

**Port work.** Route relation-level formula checking through the selected
finite backing instead of rejecting Sparse/LazySparse by name. Preserve the
selected concrete representation in explanation products and compiler
temporaries, keep explicit per-predicate representation requests outside the
favourite checker, and preserve the source Hash fallback when the selected
plugin declines a signature such as the nullary one. Verify that mutations,
clone, projection, join, union/delta and formula export are checked at the
same boundaries as the selected C++ wrapper. Do not treat an additional
Hash/Sparse cross-check as a substitute for the selected backing.

The existing focused Go test passes, but it passes by asserting the unsupported
Sparse behavior; it is evidence of the gap, not parity evidence. No code or
tests were changed during this audit.

**Further source-capability audit before implementation:** `doc`,
`bound_relation`, `interval_relation` and `karr_relation` are also accepted by
the Go parameter parser but rejected by the formula explanation scope. The C++
wrapper is generic, but each relation's signature capability and the
relation-level explanation product operations need checking before claiming
those combinations are supported. Record only combinations for which the C++
manager can actually select the wrapper and construct the required operations.
