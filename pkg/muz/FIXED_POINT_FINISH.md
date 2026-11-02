# Fixedpoint port completion checklist

This tracks completion of the whole `src/muz/` port in `pkg/muz/`, including live dependencies outside that directory. A working engine or green test is not a whole-source completion claim. Source algorithms, proofs, models, incremental behavior, diagnostics and explicit ownership must all be accounted for.

Last updated: 2026-11-02. Work is active. No engine is declared fully audited by this document.

## How to follow progress

Each completed item needs source paths, implementation/test evidence and a commit in the progress log. Unchecked means incomplete or unproven. Preserve existing encodings, safeguards and test coverage. Exclude dead source only after checking live consumers. Keep commits small and frequent; use focused native tests. The user runs checked/debug tests separately. Run the full native suite sparingly, roughly hourly.

## 1. Inventory and baseline

- [x] Enumerate every current partial or unclassified muz unit below from `port_manifest.json`.
- [x] Establish a native full-suite baseline: `go -C pkg test ./... -short -timeout=0` exited 0; log `/tmp/g3-native-port-audit.log`. This run began before the latest changes; those received focused checks separately.
- [ ] Reconcile stale manifest descriptions with current code, especially Duality; preserve partial status until independently justified.
- [ ] Inspect every source conditional, function and live consumer for each partial unit; turn each remaining obligation into an explicit implementation or audit item.
- [ ] Verify previously marked ported units against their actual consumers and proof/ownership requirements; status labels alone are insufficient.
- [x] Classify remaining CMake/README entries through their actual Go build/documentation replacements.

## 2. Shared fixedpoint infrastructure

- [ ] Close context/rule ownership, scope rollback, query transformation and engine lifecycle audits (`base/dl_context`, `dl_rule`, `dl_engine_base`).
- [ ] Verify parameter defaults, aliases, updates and engine selection against `fixedpoint_params.hpp` and engine registration.
- [ ] Close SMT-LIB/Datalog command parsing, error reporting, query state and exported result/proof/model contracts.
- [ ] Test cancellation, resource refusal and partial construction/destruction at actual allocation boundaries; preserve errors through cleanup.

## 3. PDR — first engine closure

- [ ] Audit remaining `pdr_util.cpp` methods against live consumers; implement needed normalization/diagnostic paths and document genuinely unused helpers.
  - [x] Verify focused evidence for model-satisfied disequality reduction and source swap-removal/reprocessing order.
  - [x] Verify difference/UTVPI classification, backend selection and rejection of out-of-fragment proof lemmas.
  - [x] Verify exact arithmetic normalization, owning publication, cancellation retry and disposal under a refused allocation budget.
  - [x] Verify actual large-rational root disposal while the numerical-owner execution guard is active, including repeated cleanup.
  - [ ] Close interrupted-decrement ownership coverage in `arith_normalize.go`, including any unconsumed roots and explicit cleanup continuation.
  - [x] Confirm the full caller graph for `ceil_log2` and cube formatting; exclude only genuinely unused helpers or dormant self-tests.
  - [ ] Reconcile method-level audit evidence and mark the unit complete only after the remaining checks are satisfied.
- [ ] Confirm method and ownership evidence for all PDR units, including arithmetic fragment selection and proof-derived lemmas.
- [ ] Verify recursive/nonlinear SAT and UNSAT, assumptions/background, repeated queries, witnesses, proofs and original-rule traces at the public boundary.
  - [x] Verify repeated nonlinear unreachable/reachable/unreachable queries in one SMT-LIB context for PDR, Spacer and Duality (`TestSMT2FixedpointRepeatedNonlinearQueries`).
  - [x] Verify a newly added fact changes a previously unreachable nonlinear goal to reachable after earlier queries, with repeated SAT afterward.
  - [x] Audit the post-query pop restriction against `src/muz/base/dl_context.cpp:187-195`: preserve the PDR/Spacer rejection, align its diagnostic and verify rejection leaves the scope intact; verify Duality can pop and retract its scoped fact.
- [ ] Close all PDR source obligations before declaring the engine complete.

## 4. Spacer — second engine closure

- [ ] Close `spacer_context` reach-fact selection, predecessor tags, obligation search and ownership/exception audits.
  - [x] Verify unknown-case skipping, first-false ordering, initial-fact filtering and non-completing evaluation with the existing selection regression.
  - [x] Verify all-true/all-unknown no-selection results for current and origin-state cases without model completion or temporary-reference leaks.
  - [x] Verify the direct constant-model selection path remains usable with a zero-step evaluator budget and releases its returned owner.
  - [x] Exercise origin-selection allocation failures through success and retry with reference accounting and no failed-result publication.
  - [x] Verify a nonliteral Boolean model interpretation is skipped in favor of a later false case, without leaked temporary references.
  - [ ] Audit source VERIFY/UNREACHABLE/SASSERT transport and evaluator-exception cleanup before closing reach-fact selection.
- [ ] Close `spacer_util` model evaluation, QE/MBP, array auxiliaries, appended datatype literals and source diagnostics.
- [ ] Finish slice-proof conversion for every live proof shape. Sparse/trailing/vacuous/mixed-sort binders and reverse binary resolution now have evidence, but general multi-premise mixed orientations remain unproven.
- [ ] Verify coalesce/unfold/slice composition preserves original asserted leaves, substitutions, resolution positions and checker-valid proofs.
- [ ] Close exported model, answer, counterexample and trace behavior through `spacer_dl_interface`.
- [ ] Verify exceptional cleanup and parameter restoration in projection and preprocessing.
- [ ] Close all Spacer source obligations before declaring the engine complete.

## 5. Duality — engine and external source dependency closure

- [ ] Reconcile the stale inventory for `src/duality/` and `src/muz/duality/` with current implementation and tests.
  - [x] Correct the stale muz Duality interface descriptions; keep both units partial.
  - [x] Reconcile the external `src/duality/` overview descriptions against current code and native tests.
  - [ ] Complete the method-level and ownership audit records; overview corrections do not close the units.
- [ ] Audit RPFP graph/vocabulary ownership, nodes/edges, clone/delete, assertions, dual formulas and incremental solver scopes.
- [ ] Audit lazy-annotation expansion, heuristic ordering, covering, backtracking, restarts and constrained counterexample completion.
- [ ] Audit interpolation learning, generalization, proof cores, underapproximation, Gaussian elimination and integer/array projection.
- [ ] Preserve independent cleanup/resource errors through recovery and restart paths; extend actual-path failure campaigns where coverage is missing.
- [ ] Close fixedpoint registration, preprocessing/model conversion, quantified witness reconstruction, proofs and certificate export.
- [ ] Test recursive, mutually recursive and nonlinear SAT/UNSAT plus original-clause proof/certificate validity.
- [ ] Close wrapper numerical ownership, diagnostics, profiling and all live conditional branches.
- [ ] Close all Duality source obligations before declaring the engine complete.

## 6. Remaining muz components — required for whole-port completion

- [ ] Finish BMC engine construction, bounded/quantified search, source solver selection, proofs, interfaces and ownership audits.
- [ ] Finish CLP and TAB contexts and their public engine contracts.
- [ ] Finish relational engine/context and all partial table/relation plugins listed below, including joins, projections, complements, conversions and lifecycle paths.
- [ ] Audit explanations, similarity compression, relation-manager dispatch, external relations and checking wrappers against actual consumers.
- [ ] Audit remaining transforms, dataflow and DDNF consumers, including interactions with engine-specific preprocessing.

## 7. Acceptance gates

- [ ] Every live source unit is implemented or has an explicitly justified Go/platform replacement; no unproven partial production unit remains.
- [ ] Every implemented algorithm has meaningful focused tests; add independent semantic oracles where source tests are insufficient.
- [ ] Public fixedpoint/API/SMT-LIB integration tests cover SAT, UNSAT, unknown and exception/resource distinctions for each applicable engine.
- [ ] Proofs are independently checked and their side conditions discharged; model/answer formulas are checked against original rules.
- [ ] Incremental queries, resets, engine switches, background assumptions and cancellation/resource recovery have regression coverage.
- [ ] Final source/ownership audit and manifest agree with implementation and documented deviations.
- [ ] Final native suite passes on the integrated final state; record the exact command and revision. Incorporate the user's tagged-suite findings before final completion.

## Remaining unit ledger

These are inventory obligations, not claims that the whole unit is missing. For each row: inspect code, identify live gaps, implement them, verify semantics and ownership, then update the manifest and this ledger. Paired headers share the audit but require their own interface review.

### README

- [x] `src/muz/README` (source component guide; reference only)

### base

- [x] `src/muz/base/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/base/dl_context.cpp` (partial), `src/muz/base/dl_context.h` (partial)
- [ ] `src/muz/base/dl_engine_base.h` (partial)
- [ ] `src/muz/base/dl_rule.cpp` (partial), `src/muz/base/dl_rule.h` (partial)
- [ ] `src/muz/base/fixedpoint_params.hpp` (partial)

### bmc

- [x] `src/muz/bmc/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/bmc/dl_bmc_engine.cpp` (partial), `src/muz/bmc/dl_bmc_engine.h` (partial)

### clp

- [x] `src/muz/clp/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/clp/clp_context.cpp` (partial), `src/muz/clp/clp_context.h` (partial)

### dataflow

- [x] `src/muz/dataflow/CMakeLists.txt` (Go package/import replacement verified)

### ddnf

- [x] `src/muz/ddnf/CMakeLists.txt` (Go package/import replacement verified)

### duality

- [x] `src/muz/duality/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/duality/duality_dl_interface.cpp` (partial), `src/muz/duality/duality_dl_interface.h` (partial)

### fp

- [x] `src/muz/fp/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/fp/datalog_parser.cpp` (partial), `src/muz/fp/datalog_parser.h` (partial)
- [ ] `src/muz/fp/dl_cmds.cpp` (partial), `src/muz/fp/dl_cmds.h` (partial)
- [ ] `src/muz/fp/dl_register_engine.cpp` (partial), `src/muz/fp/dl_register_engine.h` (partial)

### pdr

- [ ] `src/muz/pdr/pdr_util.cpp` (partial)

### rel

- [x] `src/muz/rel/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/rel/check_relation.cpp` (partial), `src/muz/rel/check_relation.h` (partial)
- [ ] `src/muz/rel/dl_base.cpp` (partial), `src/muz/rel/dl_base.h` (partial)
- [ ] `src/muz/rel/dl_bound_relation.cpp` (partial), `src/muz/rel/dl_bound_relation.h` (partial)
- [ ] `src/muz/rel/dl_check_table.cpp` (partial), `src/muz/rel/dl_check_table.h` (partial)
- [ ] `src/muz/rel/dl_external_relation.cpp` (partial), `src/muz/rel/dl_external_relation.h` (partial)
- [ ] `src/muz/rel/dl_finite_product_relation.cpp` (partial), `src/muz/rel/dl_finite_product_relation.h` (partial)
- [ ] `src/muz/rel/dl_interval_relation.cpp` (partial), `src/muz/rel/dl_interval_relation.h` (partial)
- [ ] `src/muz/rel/dl_lazy_table.cpp` (partial), `src/muz/rel/dl_lazy_table.h` (partial)
- [ ] `src/muz/rel/dl_mk_explanations.cpp` (partial), `src/muz/rel/dl_mk_explanations.h` (partial)
- [ ] `src/muz/rel/dl_mk_similarity_compressor.cpp` (partial), `src/muz/rel/dl_mk_similarity_compressor.h` (partial)
- [ ] `src/muz/rel/dl_product_relation.cpp` (partial), `src/muz/rel/dl_product_relation.h` (partial)
- [ ] `src/muz/rel/dl_relation_manager.h` (partial)
- [ ] `src/muz/rel/dl_sieve_relation.cpp` (partial), `src/muz/rel/dl_sieve_relation.h` (partial)
- [ ] `src/muz/rel/dl_sparse_table.cpp` (partial), `src/muz/rel/dl_sparse_table.h` (partial)
- [ ] `src/muz/rel/dl_vector_relation.h` (partial)
- [ ] `src/muz/rel/rel_context.cpp` (partial), `src/muz/rel/rel_context.h` (partial)
- [ ] `src/muz/rel/udoc_relation.cpp` (partial), `src/muz/rel/udoc_relation.h` (partial)

### spacer

- [ ] `src/muz/spacer/spacer_context.cpp` (partial), `src/muz/spacer/spacer_context.h` (partial)
- [ ] `src/muz/spacer/spacer_dl_interface.cpp` (partial), `src/muz/spacer/spacer_dl_interface.h` (partial)
- [ ] `src/muz/spacer/spacer_util.cpp` (partial), `src/muz/spacer/spacer_util.h` (partial)

### tab

- [x] `src/muz/tab/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/tab/tab_context.cpp` (partial), `src/muz/tab/tab_context.h` (partial)

### transforms

- [x] `src/muz/transforms/CMakeLists.txt` (Go package/import replacement verified)
- [ ] `src/muz/transforms/dl_mk_slice.cpp` (partial)

## Progress log

- 2026-11-02 — Created the complete component/unit checklist from the current inventory. Recent proof work includes sparse/trailing binder remapping and reverse binary resolution; all remain part of a partial slice-converter audit. Native PDR, Spacer, fixedpoint, Duality and transforms checks have passed in recent focused runs; the full native baseline exited 0. Current next action: reconcile stale Duality inventory entries, then continue concrete source-gap closure in the priority order above.

- 2026-11-02 — Corrected the muz Duality interface inventory descriptions: registration, query execution and proof/certificate export are implemented; full source and ownership closure remains partial. Focused public integration verification is recorded with the commit.
  Validation: `go -C pkg test ./parsers/smt2 -run '^TestSMT2Duality(MutualRecursionIntegerUnderapproximation|SafetyCertificate|ReachableCertificate)$' -count=1` and `go -C pkg test ./portcheck -short` both passed. Commit title: `Create fixedpoint completion checklist and reconcile Duality status`.

- 2026-11-02 — PDR utility checklist decomposed into verified algorithm evidence and explicit remaining ownership/consumer obligations. The prior checklist/inventory commit is `763135367`. Current evidence covers disequality ordering, fragment classification/backend gates, exact normalization, cancellation/retry and no-allocation temporary disposal; interrupted decrement closure remains unchecked. Source targets: `src/muz/pdr/pdr_util.cpp`, `pkg/muz/pdr/pdr_util.go`, `fragment.go`, `arith_normalize.go` and their focused tests.
  Validation: `go -C pkg test ./muz/pdr -run 'Test(PDRDisequality|PDRDifference|PDRFragment|PDRContextSelects|PropSolverRejects|NormalizeArithmetic|ArithmeticNormalization|PDRArithmeticNormalization)' -short -count=1` passed. Commit title: `Record verified PDR utility evidence and remaining obligations`.

- 2026-11-02 — PDR helper caller audit: whole-source searches find `ceil_log2` only at `pdr_util.h:44` and `pdr_util.cpp:50`. Outside its overload implementations, `pp_cube` has four calls, all inside `farkas_learner::test` (`pdr_farkas_learner.cpp:952-1003`); that static self-test has a declaration and definition but no source caller. These helpers are excluded under the live-consumer rule, not added as unused Go APIs. No runtime code changed, so no additional runtime tests were needed. Previous utility evidence commit: `98ddabdf6`.
- 2026-11-02 — Cleanup review: `arithmeticNormalization.close` uses the actual `DecRefConsumed` result to retire roots; `ast_core_delete.go` decrements the reference before native worklist destruction, while `ast_core_accessors.go` reports pre-consumption manager/handle failures. The existing refused-budget test proves normal native disposal, but does not prove every diagnostic-reentrancy or tagged destructor interruption. Keep the interrupted-decrement checklist item open; do not infer coverage from allocation refusal alone.

- 2026-11-02 — Reviewed the ten remaining muz CMake component specifications and matched their build registration to existing Go package discovery/imports. `go -C pkg list ./muz/...` succeeds; the full native suite already provides build evidence. Reclassified only build entries as platform replacements; all partial production implementation statuses remain unchanged. The source README still needs its separate documentation classification. Interrupted-decrement runtime coverage remains open.
  Validation: `go -C pkg list ./muz/...` and `go -C pkg test ./portcheck -short` passed. Commit title: `Classify remaining muz build specifications as Go replacements`.

- 2026-11-02 — Added `TestArithmeticNormalizationCleanupUnderNumericalGuard` in `pkg/muz/pdr/arith_normalize_guard_test.go`. A real large-rational AST root is consumed under the numerical owner's supported execution guard; the handle is invalid afterward and repeated cleanup succeeds. The focused native test passes. This experiment does not induce an interrupted decrement, so that separate item remains open. No artificial private-handle corruption or test-only ownership adapter was introduced.

- 2026-11-02 — Reconciled the external Duality graph/header/solver overview descriptions against current graph, interpolation and search code; source units remain partial. Classified the source muz README as a reference component guide, closing the build/documentation classification item. Previous numerical-guard regression commit: `17855c060`. No engine completion status was promoted.
  Validation: `go -C pkg test ./duality ./muz/duality -short` and `go -C pkg test ./portcheck -short` passed. Commit title: `Reconcile Duality source overviews and muz documentation inventory`.

- 2026-11-02 — Added `TestSMT2FixedpointRepeatedNonlinearQueries` in `pkg/parsers/smt2/fixedpoint_nonlinear_integer_test.go`. Each engine reuses one context and the same ascending/descending recursive relations for an UNSAT/SAT/UNSAT query sequence whose goals require two relation children. The focused native integration test passes for PDR, Spacer and Duality; other lifecycle acceptance items remain open. Previous inventory reconciliation commit: `98fdf2c9c`.
  Validation: `go -C pkg test ./parsers/smt2 -run '^TestSMT2FixedpointRepeatedNonlinearQueries$' -count=1 -timeout=45s` passed. Commit title: `Test repeated nonlinear fixedpoint query isolation`.

- 2026-11-02 — Extended `TestSMT2FixedpointRepeatedNonlinearQueries`: PDR, Spacer and Duality now must produce UNSAT/SAT/UNSAT, then SAT/SAT after a new fact is added to the same context. The focused native test passes. An attempted post-query scope-pop variant exposed the current explicit PDR/Spacer restriction in `PropositionalEngine.Pop`; it was preserved and recorded for source comparison rather than bypassed or counted as complete. Previous query-isolation commit: `ea54042a0`.
  Validation: `go -C pkg test ./parsers/smt2 -run '^TestSMT2FixedpointRepeatedNonlinearQueries$' -count=1 -timeout=45s` passed. Commit title: `Test fixedpoint result refresh after rule mutation`.

- 2026-11-02 — Audited source context push/pop and preserved the non-Duality post-query guard. Aligned Go's diagnostic with the source text. `TestSMT2FixedpointPopAfterQuerySourceContract` first exposed the diagnostic mismatch; it now verifies PDR/Spacer rejection leaves scoped facts available, while Duality pop removes the scoped fact. The repeated nonlinear query/mutation regression also passes. This closes the specific post-query scope contract item, not every scope lifecycle obligation. Previous rule-mutation commit: `b4a141839`.
  Validation: focused `TestSMT2Fixedpoint(PopAfterQuerySourceContract|RepeatedNonlinearQueries)` tests pass; commit title: `Match fixedpoint post-query pop source contract`.
  The existing `TestPropositionalEngineScopeGuards` diagnostic assertion was updated to the source wording while preserving the required rejection. Complete native `go -C pkg test ./muz/fp -short` passes.

- 2026-11-02 — Added `TestSpacerReachFactSelectionWithoutFalseCase` for current/origin cases with all-true or all-unknown model interpretations. Each case requires no selected fact, no model completion and unchanged live-reference count; the existing selection-order regression also passes. Source assertion/exception transport remains explicitly open. Previous scope-contract commit: `5e2c2e350`.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerReachFactSelection' -count=1` passed. Commit title: `Test Spacer reach selection without a false case`.

- 2026-11-02 — Added `TestSpacerReachFactSelectionWithZeroStepBudget`: a directly interpreted false case must select the correct reach fact even with evaluator max_steps=0, and closing the returned owner restores the reference baseline. The focused native test passes. The experiment confirms direct Boolean cases bypass the rewriting limit; it does not claim to exercise a caught evaluator exception, whose transport/cleanup audit stays open. Previous no-selection coverage commit: `bae0c4178`.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerReachFactSelectionWithZeroStepBudget$' -count=1` passed. Commit title: `Test Spacer constant reach selection under zero step budget`.

- 2026-11-02 — Fixed a reach-selection error-return ownership gap: an evaluation result acquired before translated-formula cleanup fails is now released before returning that failure. Added `TestSpacerOriginReachSelectionAllocationFailureAndRetry`, requiring observed allocation failures, successful selection, clean reference counts, no failed-result publication and successful retry. Complete native Spacer tests pass. The campaign covers observed allocation frontiers; it does not force the specific destructor-error branch or close the broader exception audit. Previous constant-selection commit: `f71299613`.
  Validation: focused origin-selection campaign and `go -C pkg test ./muz/spacer -short` passed. Commit title: `Close Spacer reach evaluation result on cleanup failure`.

- 2026-11-02 — Added `TestSpacerReachSelectionSkipsNonliteralModelInterpretation`: a symbolic Boolean interpretation at the first case must not be mistaken for false; selection reaches the later false case with reference accounting intact. The focused native test passes with max_steps=0. This fixture returns an unevaluated model interpretation rather than triggering rewriting, so it supplies no evaluator-exception evidence; that audit remains open. Previous ownership correction: `8ab4916ae`.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerReachSelectionSkipsNonliteralModelInterpretation$' -count=1` passed. Commit title: `Test Spacer selection skips symbolic model interpretations`.
