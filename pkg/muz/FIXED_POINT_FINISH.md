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
  - [x] Preserve background/constructor-parameter invalidation through a subsequent named PDR rule update; verify SAT/UNSAT/SAT after background replacement and context disposal after parameter replacement.
  - [x] Match PDR source-rule snapshot comparison before preprocessing; preserve covers across covered additions, with independent snapshot ownership and background/parameter invalidation guards.
  - [x] Enter and restore the PDR arithmetic proof mode when rebuilding transformers during UpdateRules, including raw-cover preservation through a covered addition.
  - [x] Reset direct-query PDR contexts after uncovered additions and changed roots while preserving configuration reconstruction boundaries.
    - [x] Implement direct Context.Reset with frame/result disposal, repeatable cleanup, preserved caller-owned answers and Boolean/arithmetic reinitialization.
    - [x] Route direct-query interface invalidation through reset while keeping configuration changes on their separate reconstruction path.
    - [x] Release retired solver pools on reset, preserve cumulative counters and verify repeated reset/solve cycles keep the live pool bounded.
  - [ ] Close expression-query clone lifecycle, including propagation of pre-query properties, cumulative statistics and model/proof ownership.
    - [x] Invalidate pre-query PDR constructor state immediately on parameter updates; test invalid settings and successful retry before any query.
    - [x] Carry pre-query PDR/QPDR covers into the expression-query engine and forward property access to the active query owner.
    - [x] Preserve PDR expression-query solver statistics across collection and clone disposal without double counting.
    - [x] Select the configured engine for command/API pre-query property access; preserve absent query-local overrides while invalidating real local/module updates.
    - [x] Preserve source PDR statistics lifetimes: retain kernel counters, sum unfoldings, take maximum depth, report only live properties/cache counters and preserve the inductive result frame during statistics reset.
    - [ ] Audit exceptional cleanup after transferring pre-query owners.
      - [x] Scan every allocation stop through first expression-query success; preserve pre-query ownership before installation and inspectable frames after failed solves, reject failed results/stale frames, retry and close each fixture with zero budget usage.
      - [x] Close a nonnil partially constructed theory rewriter when predicate-rule initialization fails.
      - [x] Verify caller cancellation and every resource-step allowance through first successful PDR expression query, including pre/post-installation owner preservation, result validity, proof mode, retry and zero-budget disposal.
      - [x] Preserve the source NNF distinction between caller cancellation and resource-limit exhaustion in the fixedpoint preprocessing path.
      - [ ] Verify independent cleanup-error transport/continuation after owner transfer.
    - [ ] Close Spacer expression-query property/statistics propagation and complete model/proof validation for both engines.
      - [x] Transfer pre-query Spacer covers to the expression-query context, route later property access to the active clone and verify stale-frame rejection.
      - [x] Complete dense allocation/cancellation scans through success, with preserved owners, retry and zero-budget disposal.
      - [x] Collect shared Spacer backend statistics once through their factory; keep virtual views source-compatible no-ops.
      - [x] Forward live Spacer expression-query context/factory/kernel counters and preserve their source lifetimes across clone disposal/reset.
      - [ ] Complete global lemma-generalizer counter/timer lifetimes and missing source context/transformer timing instrumentation; audit statistics reset across those owners.
- [ ] Verify parameter defaults, aliases, updates and engine selection against `fixedpoint_params.hpp` and engine registration.
- [ ] Close SMT-LIB/Datalog command parsing, error reporting, query state and exported result/proof/model contracts.
  - [x] Preserve source pop rejection after pre-query PDR/QPDR/Spacer context instantiation; verify an uninstantiated scope can still pop.
  - [x] Support PDR pre-query level/cover access, initial context reuse, cover inheritance and the source slicing restriction.
    - [x] Extract the existing complete PDR parameterized constructor so query and pre-query paths can share identical configuration.
    - [x] Permit source-compatible pre-initialization level/cover lookup in the PDR context.
    - [x] Enable the public PDR pre-query accessor with the shared parameter configuration.
    - [x] Reuse the pre-query context and preserve its covers when initial rules are installed; test context identity and raw frames.
      - [x] Inherit direct-context pre-query properties during Initialize, close old transformers and use freshly configured solver pools for arithmetic/proof mode.
      - [x] Exercise the public first-query path and context identity.
    - [x] Reject covers while xform.slice is enabled, before publishing any predicate frames.
- [ ] Test cancellation, resource refusal and partial construction/destruction at actual allocation boundaries; preserve errors through cleanup.
  - [x] Exercise initial PDR rule installation with existing arithmetic covers under injected allocation failures; verify no failed publication, active-context cleanup, proof-mode restoration, preserved covers and successful retry.

## 3. PDR — first engine closure

- [x] Audit remaining `pdr_util.cpp` methods against live consumers; implement needed normalization/diagnostic paths and document genuinely unused helpers.
  - [x] Verify focused evidence for model-satisfied disequality reduction and source swap-removal/reprocessing order.
  - [x] Verify difference/UTVPI classification, backend selection and rejection of out-of-fragment proof lemmas.
  - [x] Verify exact arithmetic normalization, owning publication, cancellation retry and disposal under a refused allocation budget.
  - [x] Verify actual large-rational root disposal while the numerical-owner execution guard is active, including repeated cleanup.
  - [x] Verify actual pending-manager decrement rejection preserves every unconsumed normalization root for explicit cleanup continuation, including repeated cleanup.
  - [x] Confirm the full caller graph for `ceil_log2` and cube formatting; exclude only genuinely unused helpers or dormant self-tests.
  - [x] Reconcile method-level audit evidence and the live-consumer map below; mark this utility unit ported independently of broader PDR closure.
- [ ] Confirm method and ownership evidence for all PDR units, including arithmetic fragment selection and proof-derived lemmas.
- [ ] Verify recursive/nonlinear SAT and UNSAT, assumptions/background, repeated queries, witnesses, proofs and original-rule traces at the public boundary.
  - [x] Verify repeated nonlinear unreachable/reachable/unreachable queries in one SMT-LIB context for PDR, Spacer and Duality (`TestSMT2FixedpointRepeatedNonlinearQueries`).
  - [x] Verify a newly added fact changes a previously unreachable nonlinear goal to reachable after earlier queries, with repeated SAT afterward.
  - [x] Audit the post-query pop restriction against `src/muz/base/dl_context.cpp:187-195`: preserve the PDR/Spacer rejection, align its diagnostic and verify rejection leaves the scope intact; verify Duality can pop and retract its scoped fact.
- [ ] Close all PDR source obligations before declaring the engine complete.

## 4. Spacer — second engine closure

- [ ] Close `spacer_context` reach-fact selection, predecessor tags, obligation search and ownership/exception audits.
  - [x] Verify direct tag/predecessor reads for true, false, missing and symbolic interpretations without model completion or reference leaks.
  - [x] Consolidate SelectRule error cleanup under its deferred owner so cleanup errors are joined instead of discarded.
  - [x] Verify a later concrete rule replaces an earlier true-tagged nonconcrete rule, resetting predecessor flags/reuse counts.
  - [x] Verify an earlier concrete rule wins over a later concrete fact and preserves its predecessor-use flag and reuse count.
  - [ ] Finish multi-rule preference/iteration-order and exceptional-destruction coverage.
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

- [x] `src/muz/pdr/pdr_util.cpp` (live methods and ownership audited; source consumer map below)

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
- [ ] `src/muz/spacer/spacer_dl_interface.cpp` (partial)
- [x] `src/muz/spacer/spacer_dl_interface.h` (interface declaration and owner mapping audited; implementation remains partial)
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

- 2026-11-02 — Compared `spacer_context.cpp:201-254` with `PredicateTransformer.SelectRule`. Expanded direct-model selection coverage from nine to sixteen tag/predecessor combinations, including symbolic interpretations. Removed three eager cleanup calls that discarded cleanup errors before the deferred owner ran; the existing deferred cleanup now supplies one error-joining path. Focused selection tests and complete native Spacer tests pass. Specific cleanup-error injection and multi-rule ordering remain open. Previous reach-selection ownership commit: `8ab4916ae`.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerRuleSelectionUsesDirectModelInterpretations$' -count=1` and `go -C pkg test ./muz/spacer -short` passed. Commit title: `Preserve Spacer rule selection cleanup errors`.

- 2026-11-02 — Added `TestSpacerRuleSelectionPrefersConcreteLaterRule` using a recursive rule followed by a fact through the actual context rule-update path. Both tags are true; the recursive predecessor remains unknown, so selection must replace it with the concrete fact and clear its predecessor bookkeeping. The regression checks the fixture ordering, selected rule, flags/reuse count and temporary reference baseline. The focused native test passes. Other multi-rule ordering and exceptional cleanup paths remain open. Previous selection cleanup commit: `928809d86`.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerRuleSelectionPrefersConcreteLaterRule$' -count=1` passed. Commit title: `Test Spacer preference for a later concrete rule`.

- 2026-11-02 — Extended `TestSpacerRuleSelectionPrefersConcreteLaterRule` with the complementary earlier-concrete case. A false predecessor case now requires selecting the earlier recursive rule with exactly one reused predecessor; the unknown predecessor case still requires the later fact with cleared bookkeeping. Both focused native cases pass. This verifies preference for the current candidate sequence, not equivalence between Go rule order and C++ obj_map iteration; that source-order audit remains open. Previous preference test commit: `3fc4b392a`.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerRuleSelectionPrefersConcreteLaterRule$' -count=1` passed. Commit title: `Test Spacer first concrete rule short circuit`.

### Spacer interface header audit — 2026-11-02

- [x] Constructor/destructor and private `check_reset`: `spacer_engine.go` and `spacer_rule_lifecycle.go` own the context and original/preprocessed rule snapshots; cover-before-query and rule-snapshot lifetime tests cover the boundary.
- [x] `query` / `query_from_lvl`: shared query dispatch and Spacer from-level routing.
- [x] `get_answer`, `get_ground_sat_answer`, `get_rules_along_trace`: `propositional_engine.go`, `pdr_engine.go`, `context_answer.go` and `context_trace.go`; exported objects use explicit owning return contracts.
- [x] `get_num_levels`, `get_cover_delta`, `add_cover`, `add_invariant`, `get_reachable`: typed engine/context forwarding, including pre-query covers.
- [x] `collect_statistics`, `reset_statistics`, `updt_params`: shared engine statistics and parameter revision/context replacement routing.
- [x] `get_model`, `get_proof`, `display_certificate`: model ownership/routing plus source-empty overall proof and certificate presentation hooks. These hooks are source behavior, not an implemented certificate generator.

This closes the declarations and ownership mapping of `src/muz/spacer/spacer_dl_interface.h` only. It does not close `spacer_dl_interface.cpp`, source conditional coverage, slice-proof completeness or every exceptional destructor path.

- 2026-11-02 — Spacer interface header audit completed as mapped above; header status advanced to ported, implementation status unchanged. Previous concrete-rule preference commit: `cb906d591`. Commit title: `Close Spacer fixedpoint interface header audit`.
  Validation: complete native `go -C pkg test ./muz/fp ./muz/spacer -short` and `go -C pkg test ./portcheck -short` passed. From-level entry points are `QueryExpressionFromLevel` and `QueryPredicateFromLevel`, forwarded to `Context.Solve(fromLevel, ...)`.

- 2026-11-02 — Source `dl_context.cpp:187-195` guards pop on engine existence, not just completed queries. Fixed Go Pop to account for instantiated PDR/Spacer contexts. `TestFixedpointPopRejectsInstantiatedPrequeryEngine` reproduces the Spacer pre-query bypass, verifies pop before instantiation, then requires rejection with the scope preserved after a level operation. Complete native muz/fp tests pass. The attempted PDR pre-query accessor instead reports invalid state; that distinct source-boundary gap is recorded above and remains open. Previous header audit commit: `d03bc3f29`.
  Validation: `go -C pkg test ./muz/fp -short` passed. Commit title: `Guard fixedpoint pop after prequery context creation`.

- 2026-11-02 — Source PDR interface construction (`pdr_dl_interface.cpp:35-46`) confirms pre-query context existence. Extracted `newConfiguredPDRContext` from query execution, preserving every existing parameter/default/setter and constructor-error cleanup path. Complete native muz/fp tests pass. This is a prerequisite refactor, not a claim that the pre-query gap is fixed; context access and cover inheritance are separate unchecked items above. Previous context-pop guard commit: `38733018b`.
  Validation: `go -C pkg test ./muz/fp -short` passed. Commit title: `Share complete PDR context configuration for lifecycle work`.

- 2026-11-02 — Direct PDR context property operations now accept calls before rule installation, matching `pdr_context.cpp:1565-1593`. Unknown predicates report zero levels and a true cover; adding a false cover at level two creates three frames and returns that cover. Initialized the transformer map before publishing a pre-query transformer, preventing a panic and orphaned SMT context. The regression checks values and full fixture cleanup. Public accessor construction and first-query inheritance remain open; no engine completion status changed.
  Validation: `go -C pkg test ./muz/pdr -run '^TestPDRContextPropertiesBeforeRuleInitialization$' -count=1 -v` passed. Commit title: `Support PDR context covers before rule initialization`.

- 2026-11-02 — Initial PDR rule installation now copies frame properties from pre-query transformers and closes those transformers, matching source `context::update_rules`. A fresh solver pool avoids reusing solvers constructed before arithmetic/proof-mode selection; older solvers remain owned by the manager and are disposed at context close. The arithmetic constraint-cache regression checks the raw level-two false cover, three frames, replacement/closure, a distinct solver, difference-logic selection and continued rejection of arithmetic reconfiguration after rule solvers exist. Public context reuse remains a separate unchecked item.
  Validation: `go -C pkg test ./muz/pdr -run '^TestPDRInitializeInheritsPrequeryArithmeticCover$' -count=1 -v` passed. Commit title: `Preserve PDR pre-query covers during initial rule installation`.

- 2026-11-02 — PDR/QPDR public level and cover access now constructs the fully configured context before a query. The first rule installation reuses it, preserving pre-query covers; initialization errors dispose the installed engine through its owner. The public regression supplies x=0 at level two for a relation containing zero, checks context identity, all three levels, public and raw covers after the reachable first query, and removal of the old cover after adding one. Extended pre-query pop rejection coverage to PDR/QPDR. Restored the source add_cover slicing restriction and verified rejected covers create no frames. Broader PDR lifecycle/ownership acceptance items remain open.
  Validation: the focused PDR/QPDR cover and scope regressions and `go -C pkg test ./muz/fp -short` passed. Commit title: `Integrate PDR pre-query covers with first-query lifecycle`.

- 2026-11-02 — Added `TestPDRInitialRuleAllocationFailureAndRetry` for initial rule installation with an existing level-two arithmetic cover and constraint cache. Ten sampled failure budgets before the first success at 256 verify the original transformer/cover remains published, staged contexts are released, caller proof mode is restored and retry succeeds with the cover intact. Every fixture closes with zero budget usage. This covers observed allocation boundaries, not every exceptional-destruction path. Previous public lifecycle commit: `ba75fa2a5`; direct inheritance commit: `3e7e37847`. Native fixedpoint/parser/command checks also completed successfully during integration.
  Validation: `go -C pkg test ./muz/pdr -run '^TestPDRInitialRuleAllocationFailureAndRetry$' -count=1 -v` passed; `go -C pkg test ./muz/fp ./parsers/smt2 ./cmd/g3 -short` passed. Commit title: `Test PDR initial rule allocation failure recovery`.

### PDR utility source closure — 2026-11-02

| Source methods | Live source consumers | Go implementation and evidence |
| --- | --- | --- |
| `ceil_log2` (line 50) and all `pp_cube` overloads (55–87) | No caller for `ceil_log2`; cube calls only in the uncalled `farkas_learner::test` | Excluded under the live-consumer rule; source-wide caller searches recorded earlier. |
| `reduce_disequalities` (90–160) | `pdr_context.cpp:2244`, threshold 3 after projection | `pdr_util.go` / `model_projection.go`; model-value substitution, defining equalities, original-prefix swap removal, threshold and verbosity tests, cold allocation failure/retry. |
| `test_diff_logic`: numeric/offset/negative-one recognition, inequality/equality/term checks, family checks and visitor (163–338); `is_difference_logic` / `is_utvpi_logic` (340–358) | Context arithmetic classification and `pdr_prop_solver.cpp` proof-derived lemma fragment checks | `fragment.go`; source traversal/first-rejection diagnostics, ITE/offset/signed coefficient/unsupported-family cases, backend-selection and Farkas-fragment regressions. |
| `arith_normalizer` comparison dispatch, strict-relation rewriting, least-coefficient selection and polynomial division; configuration/star wrapper and `normalize_arithmetic` (361–506) | `src/test/arith_rewriter.cpp:56,63`, after theory rewriting | `arith_normalize.go`; exact rational oracle and source-driver examples, shared application/quantifier-body rewriting, annotation preservation, scoped proof restoration, owning publication, cancellation/cold allocation retry, refused-budget disposal and real pending-manager cleanup continuation. |

Source comparison arity/type preconditions are enforced by AST construction/access and concrete operation checks; the normalization divisor is used only when strictly positive and neither zero nor one. The live normalization driver supplies theory-rewritten polynomials, as in C++; no unused STL/template implementation was added. Utility completion does not establish closure of context/search, proofs, models or every previously marked PDR unit.

- 2026-11-02 — Added `TestArithmeticNormalizationCleanupKeepsUnconsumedRootsForRetry`. A real arithmetic numeral-copy failure invokes the runtime transport while the AST manager is executing; attempting normalizer cleanup returns `ErrASTPending` and leaves all actual roots in order and alive. After the callback, explicit cleanup and repeated cleanup finish, restore the original node/reference counts, preserve the caller's formula and close the entire fixture with zero budget usage. This tests refused, unconsumed decrements rather than claiming an injected allocation budget interrupts native deletion. Reconciled the method/consumer table above and promoted only `pdr_util.cpp` from partial to ported.
  Validation: focused native utility checks (`Test(PDRDisequality|PDRDifference|PDRFragment|PDRContextSelects|PropSolverRejects|NormalizeArithmetic|ArithmeticNormalization|PDRArithmeticNormalization)`), API `TestArithmeticRewriterSourceExamples`, and port inventory checks passed. Commit title: `Close PDR utility source and ownership audit`.

- 2026-11-02 — Fixed a PDR correctness bug in named rule updates: updating a rule after changing the background unconditionally advanced the reuse revision, allowing the old background to survive. The new regression reproduced SAT after setting the background to false. UpdateRule now advances the reusable revision only when the preceding revision was already eligible, preserving background and parameter invalidation. Regressions require SAT/UNSAT/SAT across true/false/true backgrounds and fresh, closed/replaced constructor state after a cache-mode parameter change followed by a rule update. Existing strengthened-rule reuse coverage continues to pass. Source-rule comparison and covered-addition/query-root reuse remain separate open work.
  Validation: `go -C pkg test ./muz/fp ./parsers/smt2 -short` passed. Commit title: `Preserve PDR configuration invalidation across rule updates`.

- 2026-11-02 — Corrected the preceding test-file edit by restoring the existing `TestPDREngineRebuildsContextForBackgroundChange` alongside the two new revision regressions. The original independent background-change check remains in addition to the update-rule sequences; no coverage is intentionally removed.
  Validation: focused native background/configuration lifecycle regressions passed. Commit title: `Restore independent PDR background lifecycle regression`.

- 2026-11-02 — PDR now owns distinct original-rule and transformed-rule snapshots. Reuse checks original rules against the current source rules before preprocessing, as in `pdr_dl_interface::check_reset`; covered additions can reuse state without hiding earlier background/parameter invalidation. Snapshots rotate and close independently, including engine replacement/close. Added regression coverage for source-rule counts, covered-addition reuse, uncovered-addition replacement and snapshot retirement. An arithmetic covered-addition test exposed a real proof-mode gap in `Context.UpdateRules`: replacement solvers were constructed in the caller's mode rather than the context's. Scoped proof-mode entry/restoration fixes the subsequent reachability precondition failure; the regression checks the preserved raw x=0 frame and unchanged caller proof mode. Updated the stale interface manifest description without claiming whole-engine closure. Changed query-root reuse and source-compatible reset identity remain open.
  Validation: complete native PDR/fixedpoint/parser checks and port inventory checks passed. Commit title: `Preserve PDR covers across source-subsumed rule additions`.

- 2026-11-02 — Source audit of query-root changes: `rule_manager::mk_query` (`dl_rule.cpp:264-352`) always creates a fresh synthetic predicate through `context::mk_fresh_head_predicate` (`dl_context.cpp:443-455`); `check_subsumes` first compares head identity (`dl_context.cpp:522-540`). An expression-query change can therefore require a full frame reset, rather than cover preservation across arbitrary roots. Added the direct Go `Context.Reset` counterpart of `pdr_context.cpp:1467-1488`: dispose search/transformers, clear installed program and result, preserve manager/configuration/projectors and allow Initialize again. New solver pools clear obsolete arithmetic selection before reselection. Regressions check discarded covers, closed old transformers, undefined result, stale-answer rejection while a caller-owned answer stays valid, repeated reset, changed backgrounds and Boolean/arithmetic/Boolean reinitialization with proof-mode restoration. Public reset routing remains open.
  Validation: complete native PDR and fixedpoint checks passed. Commit title: `Implement reusable PDR context reset lifecycle`.

- 2026-11-02 — Integrated source-style reset for direct PDR queries after changed roots or uncovered rules, preserving the outer context while disposing old transformers, frames, preprocessing and snapshots. Background/parameter revisions still require constructor replacement; covered rules still use property inheritance. Corrected prior tests that demanded object replacement under the old Go policy: they now require the same context, closed old transformers/snapshots, discarded frames and SAT/UNSAT/SAT when changing query roots and returning. Existing raw-cover invalidation and configuration-invalidation regressions remain. Reset now closes retired Go kernels because their proof/arithmetic setup cannot be changed after search, retaining cumulative statistics instead of kernel storage. A five-round arithmetic regression verifies at most two live kernels per installed program, zero kernels/predicates/active child contexts after reset, closed retired kernels and unchanged cumulative check counts across reset and the source no-op statistics reset. Expression-query cloning remains a separate unchecked lifecycle obligation.
  Validation: complete native PDR/fixedpoint/parser checks, focused PDR SMT-LIB command scripts and port inventory checks passed. Commit title: `Reset PDR programs without replacing configured contexts`.

- 2026-11-02 — Expression-query lifecycle audit exposed a prerequisite PDR constructor bug: a context created by pre-query NumLevels survived SetQueryParameters, and further property access silently ignored new constructor settings. The regression reproduced successful access after setting invalid pdr.cache_mode=99. SetQueryParameters now invokes the PDR replacement cleanup as well as Spacer cleanup, matching `pdr_dl_interface.cpp:214-217`; the test requires rejection of the invalid setting, disposal of the old context, no failed-context publication and successful retry with a valid setting. Expression-query cover transfer and statistics remain open.
  Validation: complete native fixedpoint checks passed. Commit title: `Invalidate pre-query PDR contexts on parameter updates`.

- 2026-11-02 — PDR/QPDR expression queries now move the pre-query context into the actual query owner after preparation succeeds; level/cover operations use that owner afterward. Ordinary predicates no longer acquire false entries in the auxiliary set during cloning, fixing erroneous model-converter removal that turned a real cover into true. Engine regressions check raw/public x=0 covers, post-query cover insertion, owner transfer/disposal, fresh-query frame reset and caller-owned answer/cover survival. Added active-clone statistics forwarding and one-time archival on disposal; a three-query regression checks exact cumulative SMT-check contributions and repeatable collection/cleanup.
  Command preparation now distinguishes an absent local override from explicit parameter updates. Module updates and explicit local updates dispose obsolete PDR/Spacer/query contexts; pre-query command property calls select the configured engine. The command regression preserves a pre-query integer cover through an expression query and requires new invalid constructor settings to be reported after a module update. The API lifecycle fixture now disables slicing before exercising covers, matching the source default/guard (`fixedpoint_params.pyg:142`, `pdr_dl_interface.cpp:183-188`); a separate API regression preserves coverage of default rejection and successful cover insertion after slicing is disabled.
  Validation: native fixedpoint/command-context/parser packages, focused API fixedpoint checks and inventory checks passed. Full native suite started with log `/tmp/g3-native-expression-query.log`; completion is tracked separately. Exception campaigns, exact statistics-reset equivalence, Spacer propagation and full model/proof validation remain open. Commit title: `Integrate PDR expression-query covers and statistics`.

- 2026-11-02 — The full native run finished with exit 1 solely because `TestGeneratedFixedpointReplayFactsAndRelationQuery` selected PDR and later added covers with source-default slicing still enabled. Corrected the replay's custom engine-setup command to disable slicing; every recorded replay command and assertion remains in place. The separate API slicing-contract regression still verifies default rejection. All other packages in the full run passed; the corrected complete API package then passed. This records combined package evidence, not an exit-0 rerun of the whole-suite command.
  Validation: `go -C pkg test ./api -short` passed (36.881s); full-run log `/tmp/g3-native-expression-query.log` records the sole corrected failure. Commit title: `Apply source slicing precondition to fixedpoint replay fixture`.

- 2026-11-02 — Split PDR archived statistics according to source ownership (`pdr_context.cpp:2327-2353`): snapshots carry solver counters, cumulative unfoldings and maximum depth; transformer properties/cache counters and the current inductive level come only from the live program. Maximum depth is merged by max rather than summed. Statistics reset clears PDR search counters while preserving native kernel counters. The recursive integer expression-query regression now establishes actual search depth, checks cumulative counts and current-only property gauges across three query owners, verifies repeatable collection/disposal and exercises statistics reset. A second regression exposed the previous ResetStatistics bug that cleared a solved inductive frame (1 became 0); reset now preserves that result state, as C++ does. Rule updates also preserve cumulative node/depth counters until explicit statistics reset, verified by the existing tagged Horn-chain proof regression. Broader PDR source/ownership and expression-query exception audits remain open.
  Validation: complete native PDR/fixedpoint/command-context/parser checks and focused API fixedpoint checks passed. Commit title: `Preserve PDR statistics lifetimes across query resets`.

- 2026-11-02 — Added a dense allocation campaign for expression queries with an existing integer cover. It first exposed cover loss after owner handoff: failed preparation now returns a still-uninitialized owner to the parent, and failed solves keep installed frames inspectable without publishing a result/answer. Property ownership has a separate revision guard, so later rule mutation still rejects stale frames. The campaign requires failures both before and after rule installation, verifies the original owner identity and raw/public cover semantics, retries successfully and closes every complete fixture with zero budget usage. Scanning intermediate allocation stops also found a 240-byte leak at stop 321: NewTheoryRewriter can return a nonnil partial owner with an error; InitializeRules now closes that owner and joins cleanup errors. All allocation stops through the first native success pass. Cancellation and distinct cleanup-error campaigns remain unchecked.
  Validation: focused native PDR context/initialization tests, complete fixedpoint/command-context/parser checks, API fixedpoint checks and port inventory checks passed. Commit title: `Preserve PDR cover owners across expression-query failures`.

- 2026-11-02 — The PDR cancellation scan reached NNF preprocessing at resource step 134 and exposed a diagnostic mismatch: Go always returned "canceled", while `src/ast/normal_forms/nnf.cpp:381-386` uses the resource service's cancellation message. NNF now reports the actual reason. Its existing memory/cancellation guard regression also verifies resource-limit exhaustion with a real Push/Inc scope; the original cancellation expectation remains unchanged. This closes that dependency mismatch, not the broader NNF audit.
  Validation: complete native normal-forms package, focused SMT cancellation/resource tests and fixedpoint/command-context/parser packages passed. Commit title: `Distinguish NNF cancellation from resource exhaustion`.

- 2026-11-02 — Added `TestPDRExpressionCancellationPreservesOwnersAndRetry`. It exercises an explicit caller cancellation and every resource-step allowance through the first successful query, requiring interrupted work both before and after rule installation. It preserves caller cancellation counts and proof mode, verifies original cover/owner identity, distinguishes error from unknown result contracts, rejects unavailable answers and successfully retries without the limit. The shared covered-expression fixture keeps every prior allocation-campaign assertion and checks zero budget usage after full owner closure in both campaigns. The scan exposed and now passes the NNF diagnostic correction in `38570a6ed`. Independent cleanup-error continuation and Spacer/model/proof obligations remain open.
  Validation: native cancellation and dense allocation campaigns, complete fixedpoint/command-context/parser packages, focused API fixedpoint checks, normal-forms tests and focused SMT resource/cancellation checks passed. Commit title: `Verify PDR expression-query cancellation recovery`.

- 2026-11-02 — Spacer expression-query allocation scanning exposed virtual-solver teardown that can refuse allocation while asserting its disabled guard. Preserve the guard/assertion roots on refusal and mark that view as closing; its owning manager/factory resumes only closes that were explicitly requested, while still rejecting ordinary live views. Reject new checks on a closing view and make Manager.Close repeatable after successful disposal. The direct regression fills real shared views until guard disabling needs storage, refuses that allocation, verifies the live guard and rejected new work, then closes through the registry. Broader expression-query work is in progress; its allocation scan has reached a separate rule-normalization leak and is not yet green.
  Validation: `go -C pkg test ./muz/spacer -run '^TestSpacerManagerResumesRefusedVirtualSolverClose$' -count=1` passed. Commit title: `Resume Spacer virtual solver cleanup after allocation refusal`.

- 2026-11-02 — Spacer's expression-query allocation scan exposed an interrupted SMT assertion preprocessor being reused with unfinished rewrite frames. Assertion submission has no resume contract; failed rewriting now flushes that traversal and restores the borrowed asserted substitution before retry/teardown. The scan reproduced a teardown AST precondition failure at allocation allowance 1359 before this correction, then advanced beyond it. The separate Spacer result-owner corrections also allow the complete dense expression-query allocation scan to pass in the pending integration batch.
  Validation: `go -C pkg test ./smtcore -run 'TestSMTAssert' -short -count=1` passed, and the dense Spacer expression-query allocation scan passed through its first success. Commit title: `Reset interrupted SMT assertion preprocessing before reuse`.

- 2026-11-02 — Spacer expression queries now transfer the pre-query context after preparation and route NumLevels/CoverDelta/AddCover to the active clone, with independent property revision checks after failed solves. Failed preparation restores an uninitialized pre-query owner; failed installed solves keep inspectable frames while rejecting results/answers. An Initialized getter lets cleanup preserve pre-query properties instead of destroying their owner after initial-rule failure. The new cover regression checks raw/public integer covers, post-query insertion, repeated synthetic-query reset, disposal and caller-owned answer/cover survival.
  Dense resource-step and allocation scans through first success verify pre/post-installation ownership, proof mode/caller cancellation, failure result contracts, retry, stale-frame rejection after mutations and zero-budget disposal. Actual failures exposed discarded partial theory-rewriter/variable-substitution constructors and a leaked reachability model on deferred solver-cleanup or rule-selection error; those owners now close. Spacer's source-specific "spacer canceled" checkpoint diagnostic is preserved. Virtual-solver teardown and SMT assertion retry prerequisites were committed separately as `8d0ac6d81` and `3a7cbebe9`. Statistics propagation and complete model/proof audits remain unchecked.
  Validation: the dense allocation scan and complete native Spacer/fixedpoint/command-context/parser packages passed; focused API Spacer/expression and port inventory checks passed. The native SMT-LIB runner scripts also passed. Commit title: `Preserve Spacer expression-query covers and failure ownership`.

- 2026-11-02 — Source `spacer_virtual_solver.h:82` makes per-view statistics collection a no-op; the owning factory collects its shared backend. Go previously inherited the backend method through each virtual view, duplicating counters again when predicate transformers were collected. Added the explicit no-op and a real solved-context regression that compares context check counts with the owner/factory counts, including repeated collection. Before the fix it reported two checks for one actual check. Clone forwarding and cumulative statistics remain separate open items.
  Validation: `go -C pkg test ./muz/spacer -short -count=1` passed. Commit title: `Count Spacer shared solver statistics once`.

- 2026-11-02 — Spacer now archives source-persistent context and shared-solver statistics when expression clones or constructor contexts are replaced, forwards live clone collection/reset, and merges archived contributions only after successful disposal. Snapshots separate native kernel counters (source reset no-op) from resettable factory counters/timers and search counters. Search counts sum, maximum search depth takes max, while latest attempted query level and latest completed counterexample depth replace prior values even when zero. Current inductive level and transformer properties/propagation gauges stay with the live program.
  A recursive integer expression-query regression alternates a depth-three target with the initial fact and back. It verifies exact cumulative native/virtual check counts and search queries, max depth, latest-query values, current-only properties, repeatable collection, counter reset, parameter-driven disposal and repeated cleanup without double counting. Before forwarding it reproduced a missing parent maximum depth (0 versus 5). Global generalizer counters/timers and missing source stopwatch instrumentation remain an explicit unchecked audit, alongside model/proof closure.
  Validation: focused expression statistics, complete native Spacer/fixedpoint/command-context/parser packages, focused API fixedpoint tests and port inventory checks passed. Commit title: `Preserve Spacer statistics across expression-query replacement`.
